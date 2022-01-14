package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/ostree"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type repository struct {
	BaseURL    string `json:"baseurl,omitempty"`
	Metalink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
	GPGKey     string `json:"gpgkey,omitempty"`
	CheckGPG   bool   `json:"check_gpg,omitempty"`
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
		file, err := ioutil.ReadAll(reader)
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
		repos[i] = rpmmd.RepoConfig{
			Name:       fmt.Sprintf("repo-%d", i),
			BaseURL:    repo.BaseURL,
			Metalink:   repo.Metalink,
			MirrorList: repo.MirrorList,
			GPGKey:     repo.GPGKey,
			CheckGPG:   repo.CheckGPG,
		}
	}

	packageSets := imageType.PackageSets(composeRequest.Blueprint)

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	rpm_md := rpmmd.NewRPMMD(path.Join(home, ".cache/osbuild-composer/rpmmd"))

	packageSpecSets := make(map[string][]rpmmd.PackageSpec)
	for name, packages := range packageSets {
		packageSpecs, _, err := rpm_md.Depsolve(packages, repos, d.ModulePlatformID(), arch.Name(), d.Releasever())
		if err != nil {
			panic("Could not depsolve: " + err.Error())
		}
		packageSpecSets[name] = packageSpecs
	}

	var bytes []byte
	if rpmmdArg {
		bytes, err = json.Marshal(packageSpecSets)
		if err != nil {
			panic(err)
		}
	} else {

		if composeRequest.OSTree.Ref == "" {
			// use default OSTreeRef for image type
			composeRequest.OSTree.Ref = imageType.OSTreeRef()
		}

		manifest, err := imageType.Manifest(composeRequest.Blueprint.Customizations,
			distro.ImageOptions{
				Size: imageType.Size(0),
				OSTree: ostree.RequestParams{
					Ref:    composeRequest.OSTree.Ref,
					Parent: composeRequest.OSTree.Parent,
					URL:    composeRequest.OSTree.URL,
				},
			},
			repos,
			packageSpecSets,
			seedArg)
		if err != nil {
			panic(err.Error())
		}

		bytes, err = json.Marshal(manifest)
		if err != nil {
			panic(err)
		}
	}
	os.Stdout.Write(bytes)
}
