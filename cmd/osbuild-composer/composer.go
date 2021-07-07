package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/cloudapi"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Composer struct {
	config   *ComposerConfigFile
	stateDir string
	cacheDir string
	logger   *log.Logger
	distros  *distroregistry.Registry

	rpm rpmmd.RPMMD

	workers *worker.Server
	weldr   *weldr.API
	api     *cloudapi.Server
	koji    *kojiapi.Server

	weldrListener, localWorkerListener, workerListener, apiListener net.Listener
}

func NewComposer(config *ComposerConfigFile, stateDir, cacheDir string, logger *log.Logger) (*Composer, error) {
	c := Composer{
		config:   config,
		stateDir: stateDir,
		cacheDir: cacheDir,
		logger:   logger,
	}

	queueDir, err := c.ensureStateDirectory("jobs", 0700)
	if err != nil {
		return nil, err
	}

	artifactsDir, err := c.ensureStateDirectory("artifacts", 0755)
	if err != nil {
		return nil, err
	}

	c.distros = distroregistry.NewDefault()

	c.rpm = rpmmd.NewRPMMD(path.Join(c.cacheDir, "rpmmd"), "/usr/libexec/osbuild-composer/dnf-json")

	jobs, err := fsjobqueue.New(queueDir)
	if err != nil {
		return nil, fmt.Errorf("cannot create jobqueue: %v", err)
	}

	c.workers = worker.NewServer(c.logger, jobs, artifactsDir, c.config.WorkerAPI.IdentityFilter)

	return &c, nil
}

func (c *Composer) InitWeldr(repoPaths []string, weldrListener net.Listener) error {
	archName := common.CurrentArch()

	hostDistro := c.distros.FromHost()
	if hostDistro == nil {
		return fmt.Errorf("host distro is not supported")
	}

	arch, err := hostDistro.GetArch(archName)
	if err != nil {
		return fmt.Errorf("Host distro does not support host architecture: %v", err)
	}

	rr, err := reporegistry.New(repoPaths)
	if err != nil {
		return fmt.Errorf("error loading repository definitions: %v", err)
	}

	// Check if repositories for the host distro and arch were loaded
	_, err = rr.ReposByArch(arch, false)
	if err != nil {
		return fmt.Errorf("loaded repository definitions don't contain any for the host distro/arch: %v", err)
	}

	store := store.New(&c.stateDir, arch, c.logger)
	compatOutputDir := path.Join(c.stateDir, "outputs")

	c.weldr = weldr.New(c.rpm, arch, hostDistro, rr, c.logger, store, c.workers, compatOutputDir)

	c.weldrListener = weldrListener

	return nil
}

func (c *Composer) InitAPI(cert, key string, l net.Listener) error {
	c.api = cloudapi.NewServer(c.workers, c.rpm, c.distros)
	c.koji = kojiapi.NewServer(c.logger, c.workers, c.rpm, c.distros)

	if len(c.config.ComposerAPI.IdentityFilter) > 0 {
		c.apiListener = l
	} else {
		tlsConfig, err := createTLSConfig(&connectionConfig{
			CACertFile:     c.config.Koji.CA,
			ServerKeyFile:  key,
			ServerCertFile: cert,
			AllowedDomains: c.config.Koji.AllowedDomains,
		})
		if err != nil {
			return fmt.Errorf("Error creating TLS configuration: %v", err)
		}

		c.apiListener = tls.NewListener(l, tlsConfig)
	}

	return nil
}

func (c *Composer) InitLocalWorker(l net.Listener) {
	c.localWorkerListener = l
}

func (c *Composer) InitRemoteWorkers(cert, key string, l net.Listener) error {
	if len(c.config.WorkerAPI.IdentityFilter) > 0 {
		c.workerListener = l
	} else {
		tlsConfig, err := createTLSConfig(&connectionConfig{
			CACertFile:     c.config.Worker.CA,
			ServerKeyFile:  key,
			ServerCertFile: cert,
			AllowedDomains: c.config.Worker.AllowedDomains,
		})
		if err != nil {
			return fmt.Errorf("Error creating TLS configuration for remote worker API: %v", err)
		}

		c.workerListener = tls.NewListener(l, tlsConfig)
	}

	return nil
}

// Start Composer with all the APIs that had their respective Init*() called.
//
// Running without the weldr API is currently not supported.
func (c *Composer) Start() error {
	// sanity checks
	if c.localWorkerListener == nil && c.workerListener == nil {
		log.Fatal("neither the local worker socket nor the remote worker socket is enabled, osbuild-composer is useless without workers")
	}

	if c.apiListener == nil && c.weldrListener == nil {
		log.Fatal("neither the weldr API socket nor the composer API socket is enabled, osbuild-composer is useless without one of these APIs enabled")
	}

	if c.localWorkerListener != nil {
		go func() {
			s := &http.Server{
				ErrorLog: c.logger,
				Handler:  c.workers.Handler(),
			}
			err := s.Serve(c.localWorkerListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	if c.workerListener != nil {
		go func() {
			s := &http.Server{
				ErrorLog: c.logger,
				Handler:  c.workers.Handler(),
			}
			err := s.Serve(c.workerListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	if c.apiListener != nil {
		go func() {
			const apiRoute = "/api/composer/v1"
			const kojiRoute = "/api/composer-koji/v1"

			mux := http.NewServeMux()

			// Add a "/" here, because http.ServeMux expects the
			// trailing slash for rooted subtrees, whereas the
			// handler functions don't.
			mux.Handle(apiRoute+"/", c.api.Handler(apiRoute, c.config.ComposerAPI.IdentityFilter))
			mux.Handle(kojiRoute+"/", c.koji.Handler(kojiRoute))
			mux.Handle("/metrics", promhttp.Handler().(http.HandlerFunc))

			s := &http.Server{
				ErrorLog: c.logger,
				Handler:  mux,
			}

			err := s.Serve(c.apiListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	if c.weldrListener != nil {
		go func() {
			err := c.weldr.Serve(c.weldrListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	// wait indefinitely
	select {}
}

func (c *Composer) ensureStateDirectory(name string, perm os.FileMode) (string, error) {
	d := path.Join(c.stateDir, name)

	err := os.Mkdir(d, perm)
	if err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("cannot create state directory %s: %v", name, err)
	}

	return d, nil
}

type connectionConfig struct {
	// CA used for client certificate validation. If empty, then the CAs
	// trusted by the host system are used.
	CACertFile string

	ServerKeyFile  string
	ServerCertFile string
	AllowedDomains []string
}

func createTLSConfig(c *connectionConfig) (*tls.Config, error) {
	var roots *x509.CertPool

	if c.CACertFile != "" {
		caCertPEM, err := ioutil.ReadFile(c.CACertFile)
		if err != nil {
			return nil, err
		}

		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			panic("failed to parse root certificate")
		}
	}

	cert, err := tls.LoadX509KeyPair(c.ServerCertFile, c.ServerKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, chain := range verifiedChains {
				for _, domain := range c.AllowedDomains {
					if chain[0].VerifyHostname(domain) == nil {
						return nil
					}
				}
			}

			return errors.New("domain not in allowlist")
		},
	}, nil
}
