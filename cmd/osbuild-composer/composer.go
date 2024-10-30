package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	logrus "github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue/dbjobqueue"

	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/auth"
	"github.com/osbuild/osbuild-composer/internal/cloudapi"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/weldr"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type Composer struct {
	config   *ComposerConfigFile
	stateDir string
	cacheDir string
	logger   *log.Logger
	distros  *distrofactory.Factory
	repos    *reporegistry.RepoRegistry

	solver *dnfjson.BaseSolver

	workers *worker.Server
	weldr   *weldr.API
	api     *cloudapi.Server

	weldrListener, localWorkerListener, workerListener, apiListener, promListener net.Listener
}

func NewComposer(config *ComposerConfigFile, stateDir, cacheDir string) (*Composer, error) {
	c := Composer{
		config:   config,
		stateDir: stateDir,
		cacheDir: cacheDir,
	}

	workerConfig := worker.Config{
		BasePath:             config.Worker.BasePath,
		JWTEnabled:           config.Worker.EnableJWT,
		TenantProviderFields: config.Worker.JWTTenantProviderFields,
		WorkerWatchFreq:      time.Minute * 5,
	}

	var err error
	if config.Worker.EnableArtifacts {
		workerConfig.ArtifactsDir, err = c.ensureStateDirectory("artifacts", 0755)
		if err != nil {
			return nil, err
		}
	}

	c.distros = distrofactory.NewDefault()
	err = c.distros.RegisterAliases(config.DistroAliases)
	if err != nil {
		return nil, fmt.Errorf("failed to configure distro aliases: %v", err)
	}

	repoConfigs, err := reporegistry.LoadAllRepositories(repositoryConfigs)
	switch err.(type) {
	case *reporegistry.NoReposLoadedError:
		// When running as a service (cloud API), there are no repositories
		// configured, so this is OK. The weldr initialiser should check if
		// there are no repositories configured and behave accordingly.
		// Log for troubleshooting.
		logrus.Info(err.Error())
	case nil:
		c.repos = reporegistry.NewFromDistrosRepoConfigs(repoConfigs)
	default:
		return nil, fmt.Errorf("error loading repository definitions: %w", err)
	}

	c.solver = dnfjson.NewBaseSolver(path.Join(c.cacheDir, "rpmmd"))
	c.solver.SetDNFJSONPath(c.config.DNFJson)

	// Clean up the cache, removes unknown distros and files.
	// If no repos are configured, the cache is cleared out completely.
	var repoDistros []string
	if c.repos != nil {
		repoDistros = c.repos.ListDistros()
	}
	c.solver.CleanupOldCacheDirs(repoDistros)

	var jobs jobqueue.JobQueue
	if config.Worker.PGDatabase != "" {
		dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			config.Worker.PGUser,
			config.Worker.PGPassword,
			config.Worker.PGHost,
			config.Worker.PGPort,
			config.Worker.PGDatabase,
			config.Worker.PGSSLMode,
		)

		if config.Worker.PGMaxConns > 0 {
			dbURL += fmt.Sprintf("&pool_max_conns=%d", config.Worker.PGMaxConns)
		}

		jobs, err = dbjobqueue.New(dbURL)
		if err != nil {
			return nil, fmt.Errorf("cannot create jobqueue: %v", err)
		}
	} else {
		queueDir, err := c.ensureStateDirectory("jobs", 0700)
		if err != nil {
			return nil, err
		}
		jobs, err = fsjobqueue.New(queueDir)
		if err != nil {
			return nil, fmt.Errorf("cannot create jobqueue: %v", err)
		}
	}

	workerConfig.RequestJobTimeout, err = time.ParseDuration(config.Worker.RequestJobTimeout)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse request job timeout: %v", err)
	}
	workerConfig.WorkerTimeout, err = time.ParseDuration(config.Worker.WorkerHeartbeatTimeout)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse request worker heartbeat timeout: %v", err)
	}

	c.workers = worker.NewServer(c.logger, jobs, workerConfig)

	return &c, nil
}

func (c *Composer) InitWeldr(weldrListener net.Listener, distrosImageTypeDenylist map[string][]string) (err error) {
	c.weldr, err = weldr.New(c.repos, c.stateDir, c.solver, c.distros, c.logger, c.workers, distrosImageTypeDenylist)
	if err != nil {
		return err
	}
	c.weldrListener = weldrListener

	// Preload the Metadata for all the supported distros
	c.weldr.PreloadMetadata()
	return nil
}

func (c *Composer) InitMetricsAPI(prometheus net.Listener) {
	c.promListener = prometheus
}

func (c *Composer) InitAPI(cert, key string, enableTLS bool, enableMTLS bool, enableJWT bool, l net.Listener) error {
	config := v2.ServerConfig{
		JWTEnabled:           c.config.Koji.EnableJWT,
		TenantProviderFields: c.config.Koji.JWTTenantProviderFields,
	}

	c.api = cloudapi.NewServer(c.workers, c.distros, c.repos, config)

	if !enableTLS {
		c.apiListener = l
		return nil
	}

	// If both are off or both are on, error out
	if enableJWT == enableMTLS {
		return fmt.Errorf("API: Either mTLS or JWT authentication must be enabled")
	}

	clientAuth := tls.RequireAndVerifyClientCert
	if enableJWT {
		// jwt enabled => tls listener without client auth
		clientAuth = tls.NoClientCert
	}

	tlsConfig, err := createTLSConfig(&connectionConfig{
		CACertFile:     c.config.Koji.CA,
		ServerKeyFile:  key,
		ServerCertFile: cert,
		AllowedDomains: c.config.Koji.AllowedDomains,
		ClientAuth:     clientAuth,
	})
	if err != nil {
		return fmt.Errorf("Error creating TLS configuration: %v", err)
	}

	c.apiListener = tls.NewListener(l, tlsConfig)
	return nil
}

func (c *Composer) InitLocalWorker(l net.Listener) {
	c.localWorkerListener = l
}

func (c *Composer) InitRemoteWorkers(cert, key string, enableTLS bool, enableMTLS bool, enableJWT bool, l net.Listener) error {
	if !enableTLS {
		c.workerListener = l
		return nil
	}

	// If both are off or both are on, error out
	if enableJWT == enableMTLS {
		return fmt.Errorf("Remote worker API: Either mTLS or JWT authentication must be enabled")
	}

	clientAuth := tls.RequireAndVerifyClientCert
	if enableJWT {
		// jwt enabled => tls listener without client auth
		clientAuth = tls.NoClientCert
	}

	tlsConfig, err := createTLSConfig(&connectionConfig{
		CACertFile:     c.config.Worker.CA,
		ServerKeyFile:  key,
		ServerCertFile: cert,
		AllowedDomains: c.config.Worker.AllowedDomains,
		ClientAuth:     clientAuth,
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
	// sanity checks
	if c.localWorkerListener == nil && c.workerListener == nil {
		logrus.Fatal("neither the local worker socket nor the remote worker socket is enabled, osbuild-composer is useless without workers")
	}

	if c.apiListener == nil && c.weldrListener == nil {
		logrus.Fatal("neither the weldr API socket nor the composer API socket is enabled, osbuild-composer is useless without one of these APIs enabled")
	}

	var localWorkerAPI, remoteWorkerAPI, composerAPI, prometheusAPI *http.Server

	if c.localWorkerListener != nil {
		localWorkerAPI = &http.Server{
			ErrorLog:          c.logger,
			Handler:           c.workers.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			err := localWorkerAPI.Serve(c.localWorkerListener)
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	if c.workerListener != nil {
		handler := c.workers.Handler()
		var err error
		if c.config.Worker.EnableJWT {
			keysURLs := c.config.Worker.JWTKeysURLs
			handler, err = auth.BuildJWTAuthHandler(
				keysURLs,
				c.config.Worker.JWTKeysCA,
				c.config.Worker.JWTACLFile,
				[]string{
					"/api/image-builder-worker/v1/openapi/?$",
				},
				handler,
			)
			if err != nil {
				panic(err)
			}
		}
		remoteWorkerAPI = &http.Server{
			ErrorLog:          c.logger,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			err := remoteWorkerAPI.Serve(c.workerListener)
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	if c.apiListener != nil {
		const apiRouteV2 = "/api/image-builder-composer/v2"

		mux := http.NewServeMux()

		// Add a "/" here, because http.ServeMux expects the
		// trailing slash for rooted subtrees, whereas the
		// handler functions don't.
		mux.Handle(apiRouteV2+"/", c.api.V2(apiRouteV2))

		handler := http.Handler(mux)
		var err error
		if c.config.Koji.EnableJWT {
			keysURLs := c.config.Koji.JWTKeysURLs
			handler, err = auth.BuildJWTAuthHandler(
				keysURLs,
				c.config.Koji.JWTKeysCA,
				c.config.Koji.JWTACLFile,
				[]string{
					"/api/image-builder-composer/v2/openapi/?$",
					"/api/image-builder-composer/v2/errors/?$",
				}, mux)
			if err != nil {
				panic(err)
			}
		}

		composerAPI = &http.Server{
			ErrorLog:          c.logger,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			err := composerAPI.Serve(c.apiListener)
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	if c.promListener != nil {
		// metrics listener on another port
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler().(http.HandlerFunc))

		prometheusAPI = &http.Server{
			ErrorLog:          c.logger,
			Handler:           metricsMux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			err := prometheusAPI.Serve(c.promListener)
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	if c.weldrListener != nil {
		go func() {
			err := c.weldr.Serve(c.weldrListener)
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	sigint := make(chan os.Signal, 1)

	signal.Notify(sigint, syscall.SIGTERM)
	signal.Notify(sigint, syscall.SIGINT)

	// block until interrupted
	<-sigint

	logrus.Info("Shutting down.")

	if c.apiListener != nil {
		// First, close all listeners and then wait for all goroutines to finish.
		err := composerAPI.Shutdown(context.Background())
		c.api.Shutdown()
		if err != nil {
			panic(err)
		}
	}

	if c.promListener != nil {
		err := prometheusAPI.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}

	if c.localWorkerListener != nil {
		err := localWorkerAPI.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}

	if c.workerListener != nil {
		err := remoteWorkerAPI.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}

	if c.weldrListener != nil {
		err := c.weldr.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}

	return nil
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
	ClientAuth     tls.ClientAuthType
}

func createTLSConfig(c *connectionConfig) (*tls.Config, error) {
	var roots *x509.CertPool

	if c.CACertFile != "" {
		caCertPEM, err := os.ReadFile(c.CACertFile)
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
		ClientAuth:   c.ClientAuth,
		ClientCAs:    roots,
		MinVersion:   tls.VersionTLS12,
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
