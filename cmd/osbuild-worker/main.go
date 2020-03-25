package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
)

const RemoteWorkerPort = 8700

type ComposerClient struct {
	client   *http.Client
	hostname string
}

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

func newConnection(remoteAddress string, conf *tls.Config) (net.Conn, error) {
	if remoteAddress != "" {
		return tls.Dial("tcp", remoteAddress, conf)
	}

	// plain non-encrypted connection
	return net.Dial("unix", "/run/osbuild-composer/job.socket")

}

func NewClient(remoteAddress string, conf *tls.Config) *ComposerClient {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return newConnection(remoteAddress, conf)
			},
		},
	}
	hostname := remoteAddress

	// in case of unix domain socket, use localhost as hostname
	if hostname == "" {
		hostname = "localhost"
	}

	return &ComposerClient{client, hostname}
}

func (c *ComposerClient) AddJob() (*jobqueue.Job, error) {
	type request struct {
	}

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(request{})
	if err != nil {
		panic(err)
	}
	response, err := c.client.Post(c.createURL("/job-queue/v1/jobs"), "application/json", &b)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		rawR, _ := ioutil.ReadAll(response.Body)
		r := string(rawR)
		return nil, fmt.Errorf("couldn't create job, got %d: %s", response.StatusCode, r)
	}

	job := &jobqueue.Job{}
	err = json.NewDecoder(response.Body).Decode(job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (c *ComposerClient) UpdateJob(job *jobqueue.Job, status common.ImageBuildState, result *common.ComposeResult) error {
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(&jobqueue.JobStatus{status, result})
	if err != nil {
		panic(err)
	}
	urlPath := fmt.Sprintf("/job-queue/v1/jobs/%s/builds/%d", job.ID.String(), job.ImageBuildID)
	url := c.createURL(urlPath)
	req, err := http.NewRequest("PATCH", url, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errors.New("error setting job status")
	}

	return nil
}

func (c *ComposerClient) UploadImage(job *jobqueue.Job, reader io.Reader) error {
	// content type doesn't really matter
	url := fmt.Sprintf("http://localhost/job-queue/v1/jobs/%s/builds/%d/image", job.ID.String(), job.ImageBuildID)
	_, err := c.client.Post(url, "application/octet-stream", reader)

	return err
}

func (c *ComposerClient) createURL(path string) string {
	return "http://" + c.hostname + path
}

func handleJob(client *ComposerClient) error {
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
	var remoteAddress string
	flag.StringVar(&remoteAddress, "remote", "", "Connect to a remote composer using the specified address")
	flag.Parse()

	var conf *tls.Config
	if remoteAddress != "" {
		remoteAddress = fmt.Sprintf("%s:%d", remoteAddress, RemoteWorkerPort)

		var err error
		conf, err = createTLSConfig(&connectionConfig{
			CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
			ClientKeyFile:  "/etc/osbuild-composer/worker-key.pem",
			ClientCertFile: "/etc/osbuild-composer/worker-crt.pem",
		})
		if err != nil {
			log.Fatalf("Error creating TLS config: %v", err)
		}
	}

	client := NewClient(remoteAddress, conf)
	for {
		if err := handleJob(client); err != nil {
			log.Fatalf("Failed to handle job: " + err.Error())
		}
	}
}
