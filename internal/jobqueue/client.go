package jobqueue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/common"
)

type Client struct {
	client   *http.Client
	scheme   string
	hostname string
}

func NewClient(address string, conf *tls.Config) *Client {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: conf,
		},
	}

	var scheme string
	if conf != nil {
		scheme = "http"
	} else {
		scheme = "https"
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

	var jr addJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return nil, err
	}

	return NewJob(jr.ComposeID, jr.ImageBuildID, jr.Manifest, jr.Targets), nil
}

func (c *Client) UpdateJob(job *Job, status common.ImageBuildState, result *common.ComposeResult) error {
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(&updateJobRequest{status, result})
	if err != nil {
		panic(err)
	}
	urlPath := fmt.Sprintf("/job-queue/v1/jobs/%s/builds/%d", job.ComposeID.String(), job.ImageBuildID)
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

func (c *Client) UploadImage(job *Job, reader io.Reader) error {
	// content type doesn't really matter
	url := c.createURL(fmt.Sprintf("/job-queue/v1/jobs/%s/builds/%d/image", job.ComposeID.String(), job.ImageBuildID))
	_, err := c.client.Post(url, "application/octet-stream", reader)

	return err
}

func (c *Client) createURL(path string) string {
	return c.scheme + "://" + c.hostname + path
}
