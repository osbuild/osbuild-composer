package main

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"io/ioutil"
	"log"
	"os"
	"path"
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

func createDistroConfiguration(registryPaths []string) (rpmmd.RPMMD, distro.Distro, *distro.Registry) {
	cacheDirectory, ok := os.LookupEnv("CACHE_DIRECTORY")
	if !ok {
		log.Fatal("CACHE_DIRECTORY is not set. Is the service file missing CacheDirectory=?")
	}

	rpm := rpmmd.NewRPMMD(path.Join(cacheDirectory, "rpmmd"))

	distros, err := distro.NewDefaultRegistry(registryPaths)
	if err != nil {
		log.Fatalf("Error loading distros: %v", err)
	}

	distribution, err := distros.FromHost()
	if err != nil {
		log.Fatalf("Could not determine distro from host: " + err.Error())
	}

	return rpm, distribution, distros
}
