package main

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/google/uuid"

	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"
)

type Job struct {
	ID       uuid.UUID
	Pipeline pipeline.Pipeline
	Targets  []target.Target
}

func (job *Job) Run() error {
	cmd := exec.Command("osbuild", "--store", "/var/cache/osbuild-composer/store", "--json", "-")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = json.NewEncoder(stdin).Encode(job.Pipeline)
	if err != nil {
		return err
	}
	stdin.Close()

	var result struct {
		TreeID   string `json:"tree_id"`
		OutputID string `json:"output_id"`
	}
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	for _, target := range job.Targets {
		if target.Name != "org.osbuild.local" {
			panic("foo")
		}

		err = os.MkdirAll(target.Options.Location, 0755)
		if err != nil {
			panic(err)
		}

		cp := exec.Command("cp", "-a", "-L", "/var/cache/osbuild-composer/store/refs/" + result.OutputID, target.Options.Location)
		cp.Stderr = os.Stderr
		cp.Stdout = os.Stdout
		err = cp.Run()
		if err != nil {
			panic(err)
		}
	}

	return nil
}
