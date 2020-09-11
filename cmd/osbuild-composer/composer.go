package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"github.com/osbuild/osbuild-composer/internal/worker"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
)

type Composer struct {
	config   *ComposerConfigFile
	stateDir string
	cacheDir string
	logger   *log.Logger
	distros  *distro.Registry

	rpm rpmmd.RPMMD

	workers *worker.Server
	weldr   *weldr.API
	koji    *kojiapi.Server

	weldrListener, localWorkerListener, workerListener, kojiListener net.Listener
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

	c.distros, err = distro.NewRegistry(fedora31.New(), fedora32.New(), fedora33.New(), rhel8.New())
	if err != nil {
		return nil, fmt.Errorf("Error loading distros: %v", err)
	}

	c.rpm = rpmmd.NewRPMMD(path.Join(c.cacheDir, "rpmmd"), "/usr/libexec/osbuild-composer/dnf-json")

	// construct job types of the form osbuild:{arch} for all arches
	jobTypes := []string{"osbuild"}
	jobTypesMap := map[string]bool{}
	for _, name := range c.distros.List() {
		d := c.distros.GetDistro(name)
		for _, arch := range d.ListArches() {
			jt := "osbuild:" + arch
			if !jobTypesMap[jt] {
				jobTypesMap[jt] = true
				jobTypes = append(jobTypes, jt)
			}
		}
	}

	jobs, err := fsjobqueue.New(queueDir, jobTypes)
	if err != nil {
		return nil, fmt.Errorf("cannot create jobqueue: %v", err)
	}

	c.workers = worker.NewServer(c.logger, jobs, artifactsDir)

	return &c, nil
}

func (c *Composer) InitWeldr(repoPaths []string, weldrListener, localWorkerListener net.Listener) error {
	archName := common.CurrentArch()

	hostDistro, beta, err := c.distros.FromHost()
	if err != nil {
		return err
	}

	arch, err := hostDistro.GetArch(archName)
	if err != nil {
		return fmt.Errorf("Host distro does not support host architecture: %v", err)
	}

	// TODO: refactor to be more generic
	name := hostDistro.Name()
	if beta {
		name += "-beta"
	}

	repos, err := rpmmd.LoadRepositories(repoPaths, name)
	if err != nil {
		return fmt.Errorf("Error loading repositories for %s: %v", hostDistro.Name(), err)
	}

	store := store.New(&c.stateDir, arch, c.logger)
	compatOutputDir := path.Join(c.stateDir, "outputs")

	c.weldr = weldr.New(c.rpm, arch, hostDistro, repos[archName], c.logger, store, c.workers, compatOutputDir)

	c.weldrListener = weldrListener
	c.localWorkerListener = localWorkerListener

	return nil
}

func (c *Composer) InitKoji(cert, key string, l net.Listener) error {
	servers := make(map[string]koji.GSSAPICredentials)
	for name, creds := range c.config.Koji.Servers {
		if creds.Kerberos != nil {
			servers[name] = koji.GSSAPICredentials{
				Principal: creds.Kerberos.Principal,
				KeyTab:    creds.Kerberos.KeyTab,
			}
		}
	}

	c.koji = kojiapi.NewServer(c.logger, c.workers, c.rpm, c.distros, servers)

	tlsConfig, err := createTLSConfig(&connectionConfig{
		CACertFile:     c.config.Koji.CA,
		ServerKeyFile:  key,
		ServerCertFile: cert,
		AllowedDomains: c.config.Koji.AllowedDomains,
	})
	if err != nil {
		return fmt.Errorf("Error creating TLS configuration for Koji API: %v", err)
	}

	c.kojiListener = tls.NewListener(l, tlsConfig)

	return nil
}

func (c *Composer) InitRemoteWorkers(cert, key string, l net.Listener) error {
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

	return nil
}

// Start Composer with all the APIs that had their respective Init*() called.
//
// Running without the weldr API is currently not supported.
func (c *Composer) Start() error {
	if c.weldr == nil {
		return errors.New("weldr was not initialized")
	}

	if c.localWorkerListener != nil {
		go func() {
			err := c.workers.Serve(c.localWorkerListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	if c.workerListener != nil {
		go func() {
			err := c.workers.Serve(c.workerListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	if c.kojiListener != nil {
		go func() {
			err := c.koji.Serve(c.kojiListener)
			if err != nil {
				panic(err)
			}
		}()
	}

	return c.weldr.Serve(c.weldrListener)
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
