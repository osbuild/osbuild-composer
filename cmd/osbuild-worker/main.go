package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
)

const RemoteWorkerPort = 8700

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
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

	cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
	}, nil
}

func handleJob(client *jobqueue.Client) error {
	fmt.Println("Waiting for a new job...")
	job, err := client.AddJob()
	if err != nil {
		return err
	}

	err = client.UpdateJob(job, common.IBRunning, nil)
	if err != nil {
		return err
	}

	fmt.Printf("Running job %s\n", job.ID.String())
	result, err := job.Run(client)
	if err != nil {
		log.Printf("  Job failed: %v", err)
		return client.UpdateJob(job, common.IBFailed, result)
	}

	return client.UpdateJob(job, common.IBFinished, result)
}

func main() {
	var address string
	flag.StringVar(&address, "remote", "", "Connect to a remote composer using the specified address")
	flag.Parse()

	var client *jobqueue.Client
	if address != "" {
		address = fmt.Sprintf("%s:%d", address, RemoteWorkerPort)

		conf, err := createTLSConfig(&connectionConfig{
			CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
			ClientKeyFile:  "/etc/osbuild-composer/worker-key.pem",
			ClientCertFile: "/etc/osbuild-composer/worker-crt.pem",
		})
		if err != nil {
			log.Fatalf("Error creating TLS config: %v", err)
		}

		client = jobqueue.NewClient(address, conf)
	} else {
		client = jobqueue.NewClientUnix(address)
	}

	for {
		if err := handleJob(client); err != nil {
			log.Fatalf("Failed to handle job: " + err.Error())
		}
	}
}
