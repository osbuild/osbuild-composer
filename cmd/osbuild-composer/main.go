package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rcm"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"

	"github.com/coreos/go-systemd/activation"
)

func currentArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	} else if runtime.GOARCH == "arm64" {
		return "aarch64"
	} else if runtime.GOARCH == "ppc64le" {
		return "ppc64le"
	} else if runtime.GOARCH == "s390x" {
		return "s390x"
	} else {
		panic("unsupported architecture")
	}
}

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

	stateDir := "/var/lib/osbuild-composer"

	listeners, err := activation.ListenersWithNames()
	if err != nil {
		log.Fatalf("Could not get listening sockets: " + err.Error())
	}

	if _, exists := listeners["osbuild-composer.socket"]; !exists {
		log.Fatalf("osbuild-composer.socket doesn't exist")
	}

	composerListeners := listeners["osbuild-composer.socket"]

	if len(composerListeners) != 2 && len(composerListeners) != 3 {
		log.Fatalf("Unexpected number of listening sockets (%d), expected 2 or 3", len(composerListeners))
	}

	weldrListener := composerListeners[0]
	jobListener := composerListeners[1]

	rpm := rpmmd.NewRPMMD()
	distros := distro.NewRegistry([]string{"/etc/osbuild-composer", "/usr/share/osbuild-composer"})

	distribution, err := distros.FromHost()
	if err != nil {
		log.Fatalf("Could not determine distro from host: " + err.Error())
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	store := store.New(&stateDir, distribution, *distros)

	jobAPI := jobqueue.New(logger, store)
	weldrAPI := weldr.New(rpm, currentArch(), distribution, logger, store)

	go jobAPI.Serve(jobListener)

	// Optionally run RCM API as well as Weldr API
	if len(listeners) == 3 {
		rcmListener := composerListeners[2]
		rcmAPI := rcm.New(logger, store, rpmmd.NewRPMMD())
		go rcmAPI.Serve(rcmListener)
	}

	if remoteWorkerListeners, exists := listeners["osbuild-remote-worker.socket"]; exists {
		for _, listener := range remoteWorkerListeners {
			log.Printf("Starting remote listener\n")

			tlsConfig, err := createTLSConfig(&connectionConfig{
				CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
				ServerKeyFile:  "/etc/osbuild-composer/composer-key.pem",
				ServerCertFile: "/etc/osbuild-composer/composer-crt.pem",
			})

			if err != nil {
				log.Fatalf("TLS configuration cannot be created: " + err.Error())
			}

			listener := tls.NewListener(listener, tlsConfig)
			go jobAPI.Serve(listener)
		}
	}

	weldrAPI.Serve(weldrListener)
}
