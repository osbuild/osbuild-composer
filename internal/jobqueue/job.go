package jobqueue

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/awsupload"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type Job struct {
	ID       uuid.UUID          `json:"id"`
	Pipeline *pipeline.Pipeline `json:"pipeline"`
	Targets  []*target.Target   `json:"targets"`
}

type JobStatus struct {
	Status string `json:"status"`
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

	for _, t := range job.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			err = os.MkdirAll(options.Location, 0755)
			if err != nil {
				panic(err)
			}

			cp := exec.Command("cp", "-a", "-L", "/var/cache/osbuild-composer/store/refs/"+result.OutputID+"/.", options.Location)
			cp.Stderr = os.Stderr
			cp.Stdout = os.Stdout
			err = cp.Run()
			if err != nil {
				panic(err)
			}
		case *target.AWSTargetOptions:
			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey)
			if err != nil {
				panic(err)
			}

			_, err = a.Upload("/var/cache/osbuild-composer/store/refs/"+result.OutputID+"/image.ami", options.Bucket, options.Key)
			if err != nil {
				panic(err)
			}

			/* TODO: communicate back the AMI */
			_, err = a.Register(result.OutputID, options.Bucket, options.Key)
			if err != nil {
				panic(err)
			}
		case *target.AzureTargetOptions:
		default:
			panic("foo")
		}
	}

	return nil
}
