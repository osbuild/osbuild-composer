package jobqueue

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
)

type Job struct {
	ID         uuid.UUID          `json:"id"`
	Distro     string             `json:"distro"`
	Pipeline   *pipeline.Pipeline `json:"pipeline"`
	Targets    []*target.Target   `json:"targets"`
	OutputType string             `json:"output_type"`
}

type JobStatus struct {
	Status string                `json:"status"`
	Image  *store.Image          `json:"image"`
	Result *common.ComposeResult `json:"result"`
}

func (job *Job) Run() (*store.Image, *common.ComposeResult, error, []error) {
	distros := distro.NewRegistry([]string{"/etc/osbuild-composer", "/usr/share/osbuild-composer"})
	d := distros.GetDistro(job.Distro)
	if d == nil {
		return nil, nil, fmt.Errorf("unknown distro: %s", job.Distro), nil
	}

	build := pipeline.Build{
		Runner: d.Runner(),
	}

	buildFile, err := ioutil.TempFile("", "osbuild-worker-build-env-*")
	if err != nil {
		return nil, nil, err, nil
	}
	defer os.Remove(buildFile.Name())

	err = json.NewEncoder(buildFile).Encode(build)
	if err != nil {
		return nil, nil, fmt.Errorf("error encoding build environment: %v", err), nil
	}

	tmpStore, err := ioutil.TempDir("/var/tmp", "osbuild-store")
	if err != nil {
		return nil, nil, fmt.Errorf("error setting up osbuild store: %v", err), nil
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
		return nil, nil, fmt.Errorf("error setting up stdin for osbuild: %v", err), nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("error setting up stdout for osbuild: %v", err), nil
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("error starting osbuild: %v", err), nil
	}

	err = json.NewEncoder(stdin).Encode(job.Pipeline)
	if err != nil {
		return nil, nil, fmt.Errorf("error encoding osbuild pipeline: %v", err), nil
	}
	stdin.Close()

	var result common.ComposeResult
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return nil, nil, fmt.Errorf("error decoding osbuild output: %#v", err), nil
	}

	err = cmd.Wait()
	if err != nil {
		return nil, &result, err, nil
	}

	filename, mimeType, err := d.FilenameFromType(job.OutputType)
	if err != nil {
		return nil, &result, fmt.Errorf("cannot fetch information about output type %s: %v", job.OutputType, err), nil
	}

	var image store.Image

	var r []error

	for _, t := range job.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:

			err = runCommand("cp", "-a", "-L", tmpStore+"/refs/"+result.OutputID+"/.", options.Location)
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
				return nil, &result, err, nil
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

			_, err = a.Upload(tmpStore+"/refs/"+result.OutputID+"/image.raw.xz", options.Bucket, options.Key)
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

	return &image, &result, nil, r
}

func runCommand(command string, params ...string) error {
	cp := exec.Command(command, params...)
	cp.Stderr = os.Stderr
	cp.Stdout = os.Stdout
	return cp.Run()
}
