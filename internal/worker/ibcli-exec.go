package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/images/pkg/rpmmd"
)

type ImageBuilderArgs struct {
	Distro    string
	Arch      string
	ImageType string
	Blueprint json.RawMessage

	Repositories []rpmmd.RepoConfig

	// TODO: extend to include all options
	//  - ostree
	//  - registration
	//  - bootc
	//  - seed
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

	tmpdir, err := os.MkdirTemp("", "image-builder-manifest-")
	if err != nil {
		return nil, fmt.Errorf("image-builder manifest: failed to create temporary directory: %w", err)
	}

	// TODO: catch and log remove errors
	defer os.RemoveAll(tmpdir)

	if args.Blueprint != nil {
		// image-builder can read blueprints in JSON format and uses the extension to detect
		bpFile, err := os.Create(filepath.Join(tmpdir, "blueprint.json"))
		if err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to create blueprint file: %w", err)
		}
		defer bpFile.Close()

		if _, err := bpFile.Write(args.Blueprint); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to write blueprint: %w", err)
		}

		clArgs = append(clArgs, "--blueprint", bpFile.Name())
	}

	if len(args.Repositories) > 0 {
		// use tmpdir as the datadir, which makes image-builder search for
		// repositories under datadir/repositories and matches filename to
		// distro name
		reposDir := filepath.Join(tmpdir, "repositories")
		if err := os.Mkdir(reposDir, 0700); err != nil {
			return nil, fmt.Errorf("image-builder-manifest: failed to create repositories directory: %w", err)
		}
		reposFilePath := filepath.Join(reposDir, fmt.Sprintf("%s.json", args.Distro))
		reposFile, err := os.Create(reposFilePath)
		if err != nil {
			return nil, fmt.Errorf("image-builder-manifest: failed to create repositories file: %w", err)
		}
		defer reposFile.Close()

		repos, err := json.Marshal(args.Repositories)
		if err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to serialize repositories: %w", err)
		}

		if _, err := reposFile.Write(repos); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to write repositories: %w", err)
		}

		// Prior to v41, image-builder looked for repositories in the root of
		// the data-dir. With v41, it now looks under the repositories/
		// subdirectory. Link the file from one location to the other to cover
		// both cases.
		// https://github.com/osbuild/image-builder-cli/releases/tag/v41

		// TODO: add a condition once we have a convenient way to get the
		// version
		// NOTE: See also the ImageBuilderManifestJobResult fields in in
		// jobimpl-image-builder-manifest
		oldReposFileLocation := filepath.Join(tmpdir, fmt.Sprintf("%s.json", args.Distro))
		if err := os.Symlink(reposFilePath, oldReposFileLocation); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to symlink repos file in data-dir [%s -> %s]: %w", reposFilePath, oldReposFileLocation, err)
		}

		clArgs = append(clArgs, "--data-dir", tmpdir)
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
