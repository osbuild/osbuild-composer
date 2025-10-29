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

// TODO: remove this and the conversion functions below when
// https://issues.redhat.com/browse/HMS-9746 is resolved
type repository struct {
	Name           string   `json:"name"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	IgnoreSSL      bool     `json:"ignore_ssl,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
	PackageSets    []string `json:"package_sets,omitempty"`
}

func rpmmdRepoConfigToDiskFormat(repo rpmmd.RepoConfig) repository {
	var baseURL string
	if len(repo.BaseURLs) > 0 {
		// take only the first BaseURL - we don't really have a choice here
		baseURL = repo.BaseURLs[0]
	}

	derefBool := func(b *bool) bool {
		if b == nil {
			return false
		}
		return *b
	}

	return repository{
		Name:           repo.Name,
		BaseURL:        baseURL,
		Metalink:       repo.Metalink,
		MirrorList:     repo.MirrorList,
		GPGKeys:        repo.GPGKeys,
		CheckGPG:       derefBool(repo.CheckGPG),
		IgnoreSSL:      derefBool(repo.IgnoreSSL),
		RHSM:           repo.RHSM,
		ModuleHotfixes: repo.ModuleHotfixes,
		MetadataExpire: repo.MetadataExpire,
		ImageTypeTags:  repo.ImageTypeTags,
		PackageSets:    repo.PackageSets,
	}
}

func rpmmdRepoConfigsToDiskArchMap(repos []rpmmd.RepoConfig, arch string) map[string][]repository {
	converted := make([]repository, len(repos))
	for idx, r := range repos {
		converted[idx] = rpmmdRepoConfigToDiskFormat(r)
	}

	return map[string][]repository{
		arch: converted,
	}
}

type ImageBuilderArgs struct {
	Distro       string
	Arch         string
	ImageType    string
	Blueprint    json.RawMessage
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
		// use tmpdir/datadir/ as the datadir, which makes image-builder search
		// for repositories under tmpdir/datadir/repositories/ and matches
		// filename to distro name
		datadir := filepath.Join(tmpdir, "datadir")
		reposDir := filepath.Join(datadir, "repositories")
		if err := os.MkdirAll(reposDir, 0700); err != nil {
			return nil, fmt.Errorf("image-builder-manifest: failed to create repositories directory: %w", err)
		}
		reposFilePath := filepath.Join(reposDir, fmt.Sprintf("%s.json", args.Distro))
		reposFile, err := os.Create(reposFilePath)
		if err != nil {
			return nil, fmt.Errorf("image-builder-manifest: failed to create repositories file: %w", err)
		}
		defer reposFile.Close()

		// repository files for image-builder must be a map from architecture to a repo array
		reposArchMap := rpmmdRepoConfigsToDiskArchMap(args.Repositories, args.Arch)
		repos, err := json.Marshal(reposArchMap)
		if err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to serialize repositories: %w", err)
		}

		if _, err := reposFile.Write(repos); err != nil {
			return nil, fmt.Errorf("image-builder manifest: failed to write repositories: %w", err)
		}

		clArgs = append(clArgs, "--data-dir", datadir)
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
