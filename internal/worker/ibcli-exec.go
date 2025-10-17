package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type ImageBuilderArgs struct {
	Distro    string
	Arch      string
	ImageType string
	Blueprint json.RawMessage

	// TODO: extend to include all options
	//  - ostree
	//  - registration
	//  - bootc
	//  - seed
	//  - repositories
}

// var alias for exec.Command() that can be mocked for testing
var execCommand = exec.Command

func RunImageBuilderManifest(args ImageBuilderArgs, extraEnv []string, errorWriter io.Writer) ([]byte, error) {
	var stdoutBuffer bytes.Buffer

	clArgs := []string{
		"manifest",
		"--distro", args.Distro,
		"--arch", args.Arch,
	}

	if args.Blueprint != nil {
		// image-builder can read blueprints in JSON format and uses the extension to detect
		bpFile, err := os.CreateTemp("", "image-builder-blueprint-*.json")
		if err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to create blueprint file: %w", err)
		}
		defer os.Remove(bpFile.Name())
		defer bpFile.Close()

		if _, err := bpFile.Write(args.Blueprint); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to write blueprint: %w", err)
		}

		clArgs = append(clArgs, "--blueprint", bpFile.Name())
	}

	clArgs = append(clArgs, "--", args.ImageType)
	cmd := execCommand("image-builder", clArgs...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = errorWriter
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("image-builder manifest: call failed: %w", err)
	}

	return stdoutBuffer.Bytes(), nil
}
