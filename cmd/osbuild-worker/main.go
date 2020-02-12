package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
)

type ComposerClient struct {
	client *http.Client
}

func NewClient() *ComposerClient {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", "/run/osbuild-composer/job.socket")
			},
		},
	}
	return &ComposerClient{client}
}

func (c *ComposerClient) AddJob() (*jobqueue.Job, error) {
	type request struct {
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(request{})
	response, err := c.client.Post("http://localhost/job-queue/v1/jobs", "application/json", &b)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, errors.New("couldn't create job")
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
	json.NewEncoder(&b).Encode(&jobqueue.JobStatus{status, job.ImageBuildID, result})
	req, err := http.NewRequest("PATCH", "http://localhost/job-queue/v1/jobs/"+job.ID.String(), &b)
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
	result, err := job.Run()
	if err != nil {
		log.Printf("  Job failed: %v", err)
		return client.UpdateJob(job, common.IBFailed, result)
	}

	return client.UpdateJob(job, common.IBFinished, result)
}

func main() {
	client := NewClient()
	for {
		if err := handleJob(client); err != nil {
			log.Fatalf("Failed to handle job: " + err.Error())
		}
	}
}
