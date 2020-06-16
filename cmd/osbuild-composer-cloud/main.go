package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/cloudapi"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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
		panic(fmt.Sprintf("Failed to read root certificate %v", c.CACertFile))
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		panic(fmt.Sprintf("Failed to parse root certificate %v", c.CACertFile))
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
		log.Fatalf("TLS configuration cannot be created: %v", err.Error())
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

	var cloudListener net.Listener
	var jobListener net.Listener
	if composerListeners, exists := listeners["osbuild-composer-cloud.socket"]; exists {
		if len(composerListeners) != 2 {
			log.Fatalf("Unexpected number of listening sockets (%d), expected 2", len(composerListeners))
		}

		cloudListener = composerListeners[0]
		jobListener = tls.NewListener(composerListeners[1], tlsConfig)
	} else {
		log.Fatalf("osbuild-composer-cloud.socket doesn't exist")
	}

	var logger *log.Logger
	if verbose {
		logger = log.New(os.Stdout, "", 0)
	}

	queueDir := path.Join(stateDir, "jobs")
	err = os.Mkdir(queueDir, 0700)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("cannot create queue directory: %v", err)
	}

	jobs, err := fsjobqueue.New(queueDir, []string{"osbuild"})
	if err != nil {
		log.Fatalf("cannot create jobqueue: %v", err)
	}

	rpm := rpmmd.NewRPMMD(path.Join(cacheDirectory, "rpmmd"), "/usr/libexec/osbuild-composer/dnf-json")

	distros, err := distro.NewRegistry(fedora31.New(), fedora32.New(), rhel8.New())
	if err != nil {
		log.Fatalf("Error loading distros: %v", err)
	}

	workerServer := worker.NewServer(logger, jobs, "")
	cloudServer := cloudapi.NewServer(workerServer, rpm, distros)

	go func() {
		err := workerServer.Serve(jobListener)
		if err != nil {
			panic(err)
		}
	}()

	err = cloudServer.Serve(cloudListener)
	if err != nil {
		panic(err)
	}
}
