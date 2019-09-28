package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/google/uuid"

	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"
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

func (c *ComposerClient) AddJob() (*Job, error) {
	type request struct {
		ID string `json:"id"`
	}
	type reply struct {
		Pipeline *pipeline.Pipeline `json:"pipeline"`
		Targets  *[]target.Target   `json:"targets"`
	}

	job := &Job{
		ID: uuid.New(),
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(request{job.ID.String()})
	response, err := c.client.Post("http://localhost/job-queue/v1/jobs", "application/json", &b)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, errors.New("couldn't create job")
	}

	err = json.NewDecoder(response.Body).Decode(&reply{
		Pipeline: &job.Pipeline,
		Targets:  &job.Targets,
	})
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (c *ComposerClient) UpdateJob(job *Job, status string) error {
	type request struct {
		Status string `json:"status"`
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(&request{status})
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

func main() {
	client := NewClient()

	for {
		fmt.Println("Waiting for a new job...")
		job, err := client.AddJob()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Running job %s\n", job.ID.String())
		job.Run()

		client.UpdateJob(job, "FINISHED")
	}
}
