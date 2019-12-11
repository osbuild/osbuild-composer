package jobqueue

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
)

type Job struct {
	ID         uuid.UUID          `json:"id"`
	Pipeline   *pipeline.Pipeline `json:"pipeline"`
	Targets    []*target.Target   `json:"targets"`
	OutputType string             `json:"output_type"`
}

type JobStatus struct {
	Status string       `json:"status"`
	Image  *store.Image `json:"image"`
}

func (job *Job) Run(d distro.Distro) (*store.Image, error, []error) {
	build := pipeline.Build{
		Runner: d.Runner(),
	}

	buildFile, err := ioutil.TempFile("", "osbuild-worker-build-env-*")
	if err != nil {
		return nil, err, nil
	}
	defer os.Remove(buildFile.Name())

	err = json.NewEncoder(buildFile).Encode(build)
	if err != nil {
		return nil, err, nil
	}

	tmpStore, err := ioutil.TempDir("/var/tmp", "osbuild-store")
	if err != nil {
		return nil, err, nil
	}
	defer os.RemoveAll(tmpStore)

	cmd := exec.Command(
		"osbuild",
		"--store", tmpStore,
		"--build-env", buildFile.Name(),
		"--json", "-",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err, nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err, nil
	}

	err = cmd.Start()
	if err != nil {
		return nil, err, nil
	}

	err = json.NewEncoder(stdin).Encode(job.Pipeline)
	if err != nil {
		return nil, err, nil
	}
	stdin.Close()

	var result struct {
		TreeID   string `json:"tree_id"`
		OutputID string `json:"output_id"`
	}
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return nil, err, nil
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err, nil
	}

	filename, mimeType, err := d.FilenameFromType(job.OutputType)
	if err != nil {
		return nil, err, nil
	}

	var image store.Image

	var r []error

	for _, t := range job.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			err = runCommand("cp", "-a", "-L", "/var/cache/osbuild-composer/store/refs/"+result.OutputID+"/.", options.Location)
			if err != nil {
				r = append(r, err)
				continue
			}

			err = runCommand("chown", "-R", "_osbuild-composer:_osbuild-composer", options.Location)
			if err != nil {
				r = append(r, err)
				continue
			}

			imagePath := options.Location + "/" + filename
			file, err := os.Open(imagePath)
			if err != nil {
				r = append(r, err)
				continue
			}

			fileStat, err := file.Stat()
			if err != nil {
				return nil, err, nil
			}

			image = store.Image{
				Path: imagePath,
				Mime: mimeType,
				Size: fileStat.Size(),
			}

		case *target.AWSTargetOptions:
			a, err := awsupload.New(options.Region, options.AccessKeyID, options.SecretAccessKey)
			if err != nil {
				r = append(r, err)
				continue
			}

			if options.Key == "" {
				options.Key = job.ID.String()
			}

			_, err = a.Upload("/var/cache/osbuild-composer/store/refs/"+result.OutputID+"/image.raw.xz", options.Bucket, options.Key)
			if err != nil {
				r = append(r, err)
				continue
			}

			/* TODO: communicate back the AMI */
			_, err = a.Register(t.ImageName, options.Bucket, options.Key)
			if err != nil {
				r = append(r, err)
				continue
			}
		case *target.AzureTargetOptions:
		default:
			r = append(r, fmt.Errorf("invalid target type"))
		}
		r = append(r, nil)
	}

	return &image, nil, r
}

func runCommand(command string, params ...string) error {
	cp := exec.Command(command, params...)
	cp.Stderr = os.Stderr
	cp.Stdout = os.Stdout
	return cp.Run()
}
