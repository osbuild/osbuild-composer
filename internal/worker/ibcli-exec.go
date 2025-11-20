package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/images/pkg/customizations/subscription"
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
	Subscription *subscription.ImageOptions

	// TODO: extend to include all options
	//  - ostree
	//  - bootc
	//  - seed
}

// var alias for exec.Command() that can be mocked for testing
var execCommand = exec.Command

func RunImageBuilderManifest(args ImageBuilderArgs, extraEnv []string, errorWriter io.Writer) ([]byte, error) {
	errPrefix := "image-builder manifest"
	var stdoutBuffer bytes.Buffer

	clArgs := []string{
		"manifest",
		"--use-librepo=false", // TODO: remove once https://github.com/osbuild/osbuild/pull/2253 arrives
		"--distro", args.Distro,
		"--arch", args.Arch,
	}

	tmpdir, err := os.MkdirTemp("", "image-builder-manifest-")
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create temporary directory: %w", errPrefix, err)
	}

	// TODO: catch and log remove errors
	defer os.RemoveAll(tmpdir)

	bpArgs, err := handleBlueprint(args, tmpdir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	clArgs = append(clArgs, bpArgs...)

	repoArgs, err := handleRepositories(args, tmpdir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	clArgs = append(clArgs, repoArgs...)

	subArgs, err := handleSubscription(args, tmpdir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	clArgs = append(clArgs, subArgs...)

	clArgs = append(clArgs, "--", args.ImageType)
	cmd := execCommand("image-builder", clArgs...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = errorWriter
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: call failed: %w", errPrefix, err)
	}

	return stdoutBuffer.Bytes(), nil
}

// handleBlueprint writes the Blueprint data to a file under pardir and returns
// the appropriate command line arguments to append to the image-builder call.
func handleBlueprint(args ImageBuilderArgs, pardir string) ([]string, error) {
	if args.Blueprint == nil {
		return nil, nil
	}

	// image-builder can read blueprints in JSON format and uses the extension to detect
	bpFile, err := os.Create(filepath.Join(pardir, "blueprint.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create blueprint file: %w", err)
	}
	defer bpFile.Close()

	if _, err := bpFile.Write(args.Blueprint); err != nil {
		return nil, fmt.Errorf("failed to write blueprint: %w", err)
	}

	return []string{"--blueprint", bpFile.Name()}, nil
}

// handleRepositories writes the repository configs to a file under a
// subdirectory of pardir and returns the appropriate command line arguments to
// append to the image-builder call.
func handleRepositories(args ImageBuilderArgs, pardir string) ([]string, error) {
	if len(args.Repositories) == 0 {
		return nil, nil
	}

	// use tmpdir/datadir/ as the datadir, which makes image-builder search
	// for repositories under tmpdir/datadir/repositories/ and matches
	// filename to distro name
	datadir := filepath.Join(pardir, "datadir")
	reposDir := filepath.Join(datadir, "repositories")
	if err := os.MkdirAll(reposDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create repositories directory: %w", err)
	}
	reposFilePath := filepath.Join(reposDir, fmt.Sprintf("%s.json", args.Distro))
	reposFile, err := os.Create(reposFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create repositories file: %w", err)
	}
	defer reposFile.Close()

	// repository files for image-builder must be a map from architecture to a repo array
	reposArchMap := rpmmdRepoConfigsToDiskArchMap(args.Repositories, args.Arch)
	repos, err := json.Marshal(reposArchMap)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize repositories: %w", err)
	}

	if _, err := reposFile.Write(repos); err != nil {
		return nil, fmt.Errorf("failed to write repositories: %w", err)
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
	oldReposFileLocation := filepath.Join(datadir, fmt.Sprintf("%s.json", args.Distro))
	if err := os.Symlink(reposFilePath, oldReposFileLocation); err != nil {
		return nil, fmt.Errorf("failed to symlink repos file in data-dir [%s -> %s]: %w", reposFilePath, oldReposFileLocation, err)
	}

	return []string{"--data-dir", datadir}, nil
}

// handleSubscription writes the subscription config to a file under pardir and
// returns the appropriate command line arguments to append to the
// image-builder call.
func handleSubscription(args ImageBuilderArgs, pardir string) ([]string, error) {
	if args.Subscription == nil {
		return nil, nil
	}

	subFile, err := os.Create(filepath.Join(pardir, "subscription.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription file: %w", err)
	}
	defer subFile.Close()

	// registration file for image-builder must be a map with 'redhat' as a
	// top-level key and 'subscription' below it
	subMap := map[string]map[string]subscription.ImageOptions{"redhat": {"subscription": *args.Subscription}}
	sub, err := json.Marshal(subMap)
	if err != nil {
		return nil, fmt.Errorf("image-builder manifest: failed to serialize subscription options: %w", err)
	}

	if _, err := subFile.Write(sub); err != nil {
		return nil, fmt.Errorf("image-builder manifest: failed to write subscription options: %w", err)
	}

	return []string{"--registrations", subFile.Name()}, nil
}
