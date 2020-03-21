package jobqueue

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
)

type Job struct {
	ID           uuid.UUID         `json:"id"`
	ImageBuildID int               `json:"image_build_id"`
	Manifest     *osbuild.Manifest `json:"manifest"`
	Targets      []*target.Target  `json:"targets"`
	OutputType   string            `json:"output_type"`
}

type JobStatus struct {
	Status common.ImageBuildState `json:"status"`
	Result *common.ComposeResult  `json:"result"`
}

type TargetsError struct {
	Errors []error
}

func (e *TargetsError) Error() string {
	errString := fmt.Sprintf("%d target(s) errored:\n", len(e.Errors))

	for _, err := range e.Errors {
		errString += err.Error() + "\n"
	}

	return errString
}

func (job *Job) Run(uploader LocalTargetUploader) (*common.ComposeResult, error) {
	tmpStore, err := ioutil.TempDir("/var/tmp", "osbuild-store")
	if err != nil {
		return nil, fmt.Errorf("error setting up osbuild store: %v", err)
	}
	// FIXME: how to handle errors in defer?
	defer os.RemoveAll(tmpStore)

	cmd := exec.Command(
		"osbuild",
		"--store", tmpStore,
		"--json", "-",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdin for osbuild: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdout for osbuild: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}

	err = json.NewEncoder(stdin).Encode(job.Manifest)
	if err != nil {
		return nil, fmt.Errorf("error encoding osbuild pipeline: %v", err)
	}
	// FIXME: handle or comment this possible error
	_ = stdin.Close()

	var result common.ComposeResult
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding osbuild output: %#v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return &result, err
	}

	var r []error

	for _, t := range job.Targets {
		switch options := t.Options.(type) {
		case *target.LocalTargetOptions:
			f, err := os.Open(path.Join(tmpStore, "refs", result.OutputID, options.Filename))
			if err != nil {
				r = append(r, err)
				continue
			}

			err = uploader.UploadImage(job, f)
			if err != nil {
				r = append(r, err)
				continue
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

			_, err = a.Upload(path.Join(tmpStore, "refs", result.OutputID, options.Filename), options.Bucket, options.Key)
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
	}

	if len(r) > 0 {
		return &result, &TargetsError{r}
	}

	return &result, nil
}
