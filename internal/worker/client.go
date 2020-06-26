package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type Client struct {
	client   *http.Client
	scheme   string
	hostname string
}

type Job struct {
	Id       uuid.UUID
	Manifest distro.Manifest
	Targets  []*target.Target
}

func NewClient(address string, conf *tls.Config) *Client {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: conf,
		},
	}

	var scheme string
	// Use https if the TLS configuration is present, otherwise use http.
	if conf != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}

	return &Client{client, scheme, address}
}

func NewClientUnix(path string) *Client {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	return &Client{client, "http", "localhost"}
}

func (c *Client) AddJob() (*Job, error) {
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(addJobRequest{})
	if err != nil {
		panic(err)
	}
	response, err := c.client.Post(c.createURL("/job-queue/v1/jobs"), "application/json", &b)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		var er errorResponse
		_ = json.NewDecoder(response.Body).Decode(&er)
		return nil, fmt.Errorf("couldn't create job, got %d: %s", response.StatusCode, er.Message)
	}

	var jr addJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return nil, err
	}

	return &Job{
		jr.Id,
		jr.Manifest,
		jr.Targets,
	}, nil
}

func (c *Client) JobCanceled(job *Job) bool {
	response, err := c.client.Get(c.createURL("/job-queue/v1/jobs/" + job.Id.String()))
	if err != nil {
		return true
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return true
	}

	var jr jobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return true
	}

	return jr.Canceled
}

func (c *Client) UpdateJob(job *Job, result *OSBuildJobResult) error {
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(&updateJobRequest{result})
	if err != nil {
		panic(err)
	}
	urlPath := fmt.Sprintf("/job-queue/v1/jobs/%s", job.Id)
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

func (c *Client) UploadImage(job uuid.UUID, name string, reader io.Reader) error {
	url := c.createURL(fmt.Sprintf("/job-queue/v1/jobs/%s/artifacts/%s", job, name))
	_, err := c.client.Post(url, "application/octet-stream", reader)

	return err
}

func (c *Client) createURL(path string) string {
	return c.scheme + "://" + c.hostname + path
}
