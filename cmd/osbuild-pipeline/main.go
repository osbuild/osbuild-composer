package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/ostree"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type repository struct {
	Id           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	BaseURL      string   `json:"baseurl,omitempty"`
	Metalink     string   `json:"metalink,omitempty"`
	MirrorList   string   `json:"mirrorlist,omitempty"`
	GPGKey       string   `json:"gpgkey,omitempty"`
	CheckGPG     bool     `json:"check_gpg,omitempty"`
	CheckRepoGPG bool     `json:"repo_check_gpg,omitempty"`
	IgnoreSSL    bool     `json:"ignore_ssl,omitempty"`
	PackageSets  []string `json:"package_sets,omitempty"`
	RHSM         bool     `json:"rhsm,omitempty"`
}

type ostreeOptions struct {
	Ref    string `json:"ref"`
	Parent string `json:"parent"`
	URL    string `json:"url"`
}

type composeRequest struct {
	Distro       string              `json:"distro"`
	Arch         string              `json:"arch"`
	ImageType    string              `json:"image-type"`
	Blueprint    blueprint.Blueprint `json:"blueprint"`
	Repositories []repository        `json:"repositories"`
	OSTree       ostreeOptions       `json:"ostree"`
}

// osbuild-pipeline is a utility command and is often run from within the
// source tree.  Find the dnf-json binary in case the osbuild-composer package
// isn't installed.  This prioritises the local source version over the system
// version if run from within the source tree.
func findDnfJsonBin() string {
	locations := []string{"./dnf-json", "/usr/libexec/osbuild-composer/dnf-json", "/usr/lib/osbuild-composer/dnf-json"}
	for _, djPath := range locations {
		_, err := os.Stat(djPath)
		if !os.IsNotExist(err) {
			return djPath
		}
	}

	// can't run: panic
	panic(fmt.Sprintf("could not find 'dnf-json' in any of the known paths: %+v", locations))
}

func resolveContainers(sourceSpecs []container.SourceSpec, archName string) ([]container.Spec, error) {
	if len(sourceSpecs) == 0 {
		return nil, nil
	}

	resolver := container.NewResolver(archName)

	for _, c := range sourceSpecs {
		resolver.Add(c)
	}

	return resolver.Finish()
}

func main() {
	var rpmmdArg bool
	flag.BoolVar(&rpmmdArg, "rpmmd", false, "output rpmmd struct instead of pipeline manifest")
	var seedArg int64
	flag.Int64Var(&seedArg, "seed", 0, "seed for generating manifests (default: 0)")
	flag.Parse()

	// Path to composeRequet or '-' for stdin
	composeRequestArg := flag.Arg(0)

	composeRequest := &composeRequest{}
	if composeRequestArg != "" {
		var reader io.Reader
		if composeRequestArg == "-" {
			reader = os.Stdin
		} else {
			var err error
			reader, err = os.Open(composeRequestArg)
			if err != nil {
				panic("Could not open compose request: " + err.Error())
			}
		}
		file, err := io.ReadAll(reader)
		if err != nil {
			panic("Could not read compose request: " + err.Error())
		}
		err = json.Unmarshal(file, &composeRequest)
		if err != nil {
			panic("Could not parse blueprint: " + err.Error())
		}
	}

	distros := distroregistry.NewDefault()
	d := distros.GetDistro(composeRequest.Distro)
	if d == nil {
		_, _ = fmt.Fprintf(os.Stderr, "The provided distribution '%s' is not supported. Use one of these:\n", composeRequest.Distro)
		for _, d := range distros.List() {
			_, _ = fmt.Fprintln(os.Stderr, " *", d)
		}
		return
	}

	arch, err := d.GetArch(composeRequest.Arch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided architecture '%s' is not supported by %s. Use one of these:\n", composeRequest.Arch, d.Name())
		for _, a := range d.ListArches() {
			_, _ = fmt.Fprintln(os.Stderr, " *", a)
		}
		return
	}

	imageType, err := arch.GetImageType(composeRequest.ImageType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided image type '%s' is not supported by %s for %s. Use one of these:\n", composeRequest.ImageType, d.Name(), arch.Name())
		for _, t := range arch.ListImageTypes() {
			_, _ = fmt.Fprintln(os.Stderr, " *", t)
		}
		return
	}

	repos := make([]rpmmd.RepoConfig, len(composeRequest.Repositories))
	for i, repo := range composeRequest.Repositories {
		repoName := repo.Name
		if repoName == "" {
			repoName = fmt.Sprintf("repo-%d", i)
		}
		repoId := repo.Id
		if repoId == "" {
			repoId = fmt.Sprintf("repo-%d", i)
		}
		var urls []string
		if repo.BaseURL != "" {
			urls = []string{repo.BaseURL}
		}
		var keys []string
		if repo.GPGKey != "" {
			keys = []string{repo.GPGKey}
		}

		repos[i] = rpmmd.RepoConfig{
			Id:           repoId,
			Name:         repoName,
			BaseURLs:     urls,
			Metalink:     repo.Metalink,
			MirrorList:   repo.MirrorList,
			GPGKeys:      keys,
			CheckGPG:     &repo.CheckGPG,
			CheckRepoGPG: common.ToPtr(false),
			IgnoreSSL:    false,
			PackageSets:  repo.PackageSets,
			RHSM:         repo.RHSM,
		}
	}

	if composeRequest.OSTree.Ref == "" {
		// use default OSTreeRef for image type
		composeRequest.OSTree.Ref = imageType.OSTreeRef()
	}

	options := distro.ImageOptions{
		Size: imageType.Size(0),
		OSTree: &ostree.ImageOptions{
			ImageRef:      composeRequest.OSTree.Ref,
			FetchChecksum: composeRequest.OSTree.Parent,
			URL:           composeRequest.OSTree.URL,
		},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch.Name(), d.Name(), path.Join(home, ".cache/osbuild-composer/rpmmd"))
	solver.SetDNFJSONPath(findDnfJsonBin())

	// Set cache size to 3 GiB
	// osbuild-pipeline is often used to generate a lot of manifests in a row
	// let the cache grow to fit much more repository metadata than we usually allow
	solver.SetMaxCacheSize(3 * 1024 * 1024 * 1024)

	manifest, _, err := imageType.Manifest(&composeRequest.Blueprint, options, repos, seedArg)
	if err != nil {
		panic(err.Error())
	}

	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range manifest.Content.PackageSets {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			panic("Could not depsolve: " + err.Error())
		}
		depsolvedSets[name] = res
	}

	containers := make(map[string][]container.Spec, len(manifest.Content.Containers))
	for name, sourceSpecs := range manifest.Content.Containers {
		containerSpecs, err := resolveContainers(sourceSpecs, arch.Name())
		if err != nil {
			panic("Could not resolve containers: " + err.Error())
		}
		containers[name] = containerSpecs
	}

	var bytes []byte
	if rpmmdArg {
		bytes, err = json.Marshal(depsolvedSets)
		if err != nil {
			panic(err)
		}
	} else {
		if composeRequest.OSTree.Ref == "" {
			// use default OSTreeRef for image type
			composeRequest.OSTree.Ref = imageType.OSTreeRef()
		}

		ms, err := manifest.Serialize(depsolvedSets, containers)
		if err != nil {
			panic(err.Error())
		}

		bytes, err = json.Marshal(ms)
		if err != nil {
			panic(err)
		}
	}
	os.Stdout.Write(bytes)
	if err := solver.CleanCache(); err != nil {
		// print to stderr but don't exit with error
		fmt.Fprintf(os.Stderr, "Error during rpm repo cache cleanup: %s", err.Error())
	}
}
