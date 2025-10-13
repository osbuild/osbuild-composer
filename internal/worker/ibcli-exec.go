package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/osbuild/blueprint/pkg/blueprint"
)

type ImageBuilderArgs struct {
	Distro    string
	Arch      string
	ImageType string
	Blueprint *blueprint.Blueprint

	// TODO: extend to include all options
	//  - ostree
	//  - registration
	//  - bootc
	//  - seed
	//  - repositories
}

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

		bp, err := json.Marshal(args.Blueprint)
		if err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to serialize blueprint: %w", err)
		}

		if _, err := bpFile.Write(bp); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to write blueprint: %w", err)
		}
		if err := bpFile.Close(); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to close blueprint file: %w", err)
		}

		// TODO: catch and log remove errors
		defer os.Remove(bpFile.Name())

		clArgs = append(clArgs, "--blueprint", bpFile.Name())
	}

	clArgs = append(clArgs, "--", args.ImageType)
	cmd := exec.Command("image-builder", clArgs...)
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
