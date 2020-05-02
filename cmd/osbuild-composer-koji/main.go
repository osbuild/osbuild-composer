package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"

	ComposerKoji "github.com/osbuild/osbuild-composer/internal/composer-koji"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel81"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel83"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"

	"github.com/coreos/go-systemd/activation"
)

type connectionConfig struct {
	CACertFile     string
	ServerKeyFile  string
	ServerCertFile string
}

func createTLSConfig(c *connectionConfig) (*tls.Config, error) {
	caCertPEM, err := ioutil.ReadFile(c.CACertFile)
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		panic("failed to parse root certificate")
	}

	cert, err := tls.LoadX509KeyPair(c.ServerCertFile, c.ServerKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
	}, nil
}

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "v", false, "Print access log")
	flag.Parse()

	tlsConfig, err := createTLSConfig(&connectionConfig{
		CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
		ServerKeyFile:  "/etc/osbuild-composer/composer-key.pem",
		ServerCertFile: "/etc/osbuild-composer/composer-crt.pem",
	})
	if err != nil {
		log.Fatalf("TLS configuration cannot be created: " + err.Error())
	}

	stateDir, ok := os.LookupEnv("STATE_DIRECTORY")
	if !ok {
		log.Fatal("STATE_DIRECTORY is not set. Is the service file missing StateDirectory=?")
	}

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	listeners, err := activation.ListenersWithNames()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	var kojiListener net.Listener
	var jobListener net.Listener
	if composerListeners, exists := listeners["osbuild-composer-koji.socket"]; exists {
		if len(composerListeners) != 2 {
			log.Fatalf("Unexpected number of listening sockets (%d), expected 2", len(composerListeners))
		}

		kojiListener = composerListeners[0]
		jobListener = tls.NewListener(composerListeners[1], tlsConfig)
	} else {
		log.Fatalf("osbuild-composer.socket doesn't exist")
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	rpm := rpmmd.NewRPMMD(path.Join(cacheDirectory, "rpmmd"))

	distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New(), rhel83.New())
	if err != nil {
		log.Fatalf("Error loading distros: %v", err)
	}

	store := store.New(&stateDir)
	workerAPI := worker.NewServer(logger, store)
	kojiAPI := ComposerKoji.New(store, rpm, distros)

	go func() {
		err := workerAPI.Serve(jobListener)
		if err != nil {
			panic(err)
		}
	}()

	err = kojiAPI.Serve(kojiListener)
	if err != nil {
		panic(err)
	}
}
