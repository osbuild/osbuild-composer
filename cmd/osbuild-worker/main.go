package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/upload/koji"
)

const configFile = "/etc/osbuild-worker/osbuild-worker.toml"
const backoffDuration = time.Second * 10

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
}

type kojiServer struct {
	creds              koji.GSSAPICredentials
	relaxTimeoutFactor uint
}

func createTLSConfig(config *connectionConfig) (*tls.Config, error) {
	caCertPEM, err := ioutil.ReadFile(config.CACertFile)
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		return nil, errors.New("failed to append root certificate")
	}

	var certs []tls.Certificate
	if config.ClientKeyFile != "" && config.ClientCertFile != "" {
		cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: certs,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func main() {
	var unix bool
	flag.BoolVar(&unix, "unix", false, "Interpret 'address' as a path to a unix domain socket instead of a network address")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-unix] address\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	address := flag.Arg(0)
	if address == "" {
		flag.Usage()
		os.Exit(2)
	}

	config, err := parseConfig(configFile)
	if err != nil {
		logrus.Fatalf("Could not load config file '%s': %v", configFile, err)
	}

	logrus.Info("Composer configuration:")
	encoder := toml.NewEncoder(logrus.StandardLogger().WriterLevel(logrus.InfoLevel))
	err = encoder.Encode(&config)
	if err != nil {
		logrus.Fatalf("Could not print config: %v", err)
	}

	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		logrus.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	worker, err := NewWorker(config, unix, address, cacheDirectory)
	if err != nil {
		logrus.Fatalf("%v", err)
	}

	err = worker.Start()
	if err != nil {
		logrus.Fatalf("%v", err)
	}
}
