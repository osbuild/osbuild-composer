package worker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Client struct {
	api *api.Client
}

type Job struct {
	Id       uuid.UUID
	Manifest distro.Manifest
	Targets  []*target.Target
}

func NewClient(baseURL string, conf *tls.Config) (*Client, error) {
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: conf,
		},
	}

	c, err := api.NewClient(baseURL, api.WithHTTPClient(&httpClient))
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

func NewClientUnix(path string) *Client {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	c, err := api.NewClient("http://localhost", api.WithHTTPClient(&httpClient))
	if err != nil {
		panic(err)
	}

	return &Client{c}
}

func (c *Client) AddJob() (*Job, error) {
	response, err := c.api.PostJobQueueV1Jobs(context.Background(), api.PostJobQueueV1JobsJSONRequestBody{})
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
	response, err := c.api.GetJobQueueV1JobsJobId(context.Background(), job.Id.String())
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

func (c *Client) UpdateJob(job *Job, status common.ImageBuildState, result *osbuild.Result) error {
	response, err := c.api.PatchJobQueueV1JobsJobId(context.Background(), job.Id.String(), api.PatchJobQueueV1JobsJobIdJSONRequestBody{
		Result: result,
		Status: status.ToString(),
	})
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return errors.New("error setting job status")
	}

	return nil
}

func (c *Client) UploadImage(job uuid.UUID, name string, reader io.Reader) error {
	_, err := c.api.PostJobQueueV1JobsJobIdArtifactsNameWithBody(context.Background(),
		job.String(), name, "application/octet-stream", reader)

	return err
}
