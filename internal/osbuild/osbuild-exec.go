package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type jobIdStageImpl struct {
	Type    string            `json:"type"`
	Options jobIdStageOptions `json:"options"`
}

type jobIdStageOptions struct {
	ImageId string `json:"image_id"`
}

type JSONObject = map[string]interface{}
type JSONArray = []interface{}

// Inject JobID in specific stages
//
// By the time the JobID value is available the manifest is already assembled
// and passed through as opaque byte slice, that means that we'll need to do
// a bit of reverse engineering to make the necessary changes:
//
//  1) New stage needs to be added with the type "org.osbuild.os-release.image_id"
//  2) If a stage with the type "org.osbuild.rhsm.facts" exists then the list of facts
//     is updated with the JobID value as "image-builder.osbuild-composer.image_id" fact
//     in accordance with current naming in RHSMFacts
//
// As such, this code expects the manifest to look like this:
//
//  {
//    "pipelines": [
//       {
//         "name": "os",
//         "stages": [
//           {"type": ..., "options": ...}
//         ]
//       }
//    ]
//  }
//
// Any deviation will result in an error.
//
func injectJobDetailsStageIntoManifest(manifest []byte, jobId string) ([]byte, error) {
	var m JSONObject

	if err := json.Unmarshal(manifest, &m); err != nil {
		return nil, fmt.Errorf("Failed to decode the manifest: %v", err)
	}

	pipelines := m["pipelines"].(JSONArray)
	for _, pipeline := range pipelines {
		if pipeline.(JSONObject)["name"] == "os" {
			stages := pipeline.(JSONObject)["stages"].(JSONArray)

			for _, stage := range stages {
				s := stage.(JSONObject)

				// Update RHSMFacts with image_id
				if s["type"] == "org.osbuild.rhsm.facts" {
					s["options"].(JSONObject)["facts"].(JSONObject)["image-builder.osbuild-composer.image_id"] = jobId
				}
			}

			newStage := jobIdStageImpl{
				Type: "org.osbuild.os-release.image_id",
				Options: jobIdStageOptions{
					ImageId: jobId,
				},
			}

			pipeline.(JSONObject)["stages"] = append(stages, &newStage)

			return json.Marshal(&m)
		}
	}

	return nil, fmt.Errorf("Failed to inject jobId into the manifest: no 'os' pipeline")
}

// Run an instance of osbuild, returning a parsed osbuild.Result.
//
// Note that osbuild returns non-zero when the pipeline fails. This function
// does not return an error in this case. Instead, the failure is communicated
// with its corresponding logs through osbuild.Result.
func RunOSBuild(manifest []byte, jobId, store, outputDirectory string, exports, checkpoints, extraEnv []string, result bool, errorWriter io.Writer) (*Result, error) {
	var stdoutBuffer bytes.Buffer
	var res Result

	newManifest, err := injectJobDetailsStageIntoManifest(manifest, jobId)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
		"-",
	)

	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}

	for _, checkpoint := range checkpoints {
		cmd.Args = append(cmd.Args, "--checkpoint", checkpoint)
	}

	if result {
		cmd.Args = append(cmd.Args, "--json")
		cmd.Stdout = &stdoutBuffer
	} else {
		cmd.Stdout = os.Stdout
	}

	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	cmd.Stderr = errorWriter
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdin for osbuild: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}

	_, err = stdin.Write(newManifest)
	if err != nil {
		return nil, fmt.Errorf("error writing osbuild manifest: %v", err)
	}

	err = stdin.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing osbuild's stdin: %v", err)
	}

	err = cmd.Wait()

	if result {
		// try to decode the output even though the job could have failed
		decodeErr := json.Unmarshal(stdoutBuffer.Bytes(), &res)
		if decodeErr != nil {
			return nil, fmt.Errorf("error decoding osbuild output: %v\nthe raw output:\n%s", decodeErr, stdoutBuffer.String())
		}
	}

	if err != nil {
		// ignore ExitError if output could be decoded correctly
		if _, isExitError := err.(*exec.ExitError); !isExitError {
			return nil, fmt.Errorf("running osbuild failed: %v", err)
		}
	}

	return &res, nil
}
