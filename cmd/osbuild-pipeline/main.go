package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type repository struct {
	BaseURL    string `json:"baseurl,omitempty"`
	Metalink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
	GPGKey     string `json:"gpgkey,omitempty"`
}

type composeRequest struct {
	Distro       string              `json:"distro"`
	Arch         string              `json:"arch"`
	ImageType    string              `json:"image-type"`
	Blueprint    blueprint.Blueprint `json:"blueprint"`
	Repositories []repository        `json:"repositories"`
}

type rpmMD struct {
	BuildPackages []rpmmd.PackageSpec `json:"build-packages"`
	Packages      []rpmmd.PackageSpec `json:"packages"`
	Checksums     map[string]string   `json:"checksums"`
}

func main() {
	var rpmmdArg bool
	flag.BoolVar(&rpmmdArg, "rpmmd", false, "output rpmmd struct instead of pipeline manifest")
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

	distros, err := distro.NewRegistry(fedora31.New(), fedora32.New(), rhel8.New())
	if err != nil {
		panic(err)
	}

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
		}
	}

	packages := make([]string, len(composeRequest.Blueprint.Packages))
	for i, pkg := range composeRequest.Blueprint.Packages {
		packages[i] = pkg.Name
		// If a package has version "*" the package name suffix must be equal to "-*-*.*"
		// Using just "-*" would find any other package containing the package name
		if pkg.Version != "" && pkg.Version != "*" {
			packages[i] += "-" + pkg.Version
		} else if pkg.Version == "*" {
			packages[i] += "-*-*.*"
		}
	}

	pkgs, excludePkgs := imageType.BasePackages()
	packages = append(pkgs, packages...)

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	rpmmd := rpmmd.NewRPMMD(path.Join(home, ".cache/osbuild-composer/rpmmd"))
	packageSpecs, checksums, err := rpmmd.Depsolve(packages, excludePkgs, repos, d.ModulePlatformID(), arch.Name())
	if err != nil {
		panic("Could not depsolve: " + err.Error())
	}

	buildPkgs := imageType.BuildPackages()
	buildPackageSpecs, _, err := rpmmd.Depsolve(buildPkgs, nil, repos, d.ModulePlatformID(), arch.Name())
	if err != nil {
		panic("Could not depsolve build packages: " + err.Error())
	}

	var bytes []byte
	if rpmmdArg {
		rpmMDInfo := rpmMD{
			BuildPackages: buildPackageSpecs,
			Packages:      packageSpecs,
			Checksums:     checksums,
		}
		bytes, err = json.Marshal(rpmMDInfo)
		if err != nil {
			panic(err)
		}
	} else {
		manifest, err := imageType.Manifest(composeRequest.Blueprint.Customizations,
			distro.ImageOptions{
				Size: imageType.Size(0),
			},
			repos,
			packageSpecs,
			buildPackageSpecs)
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
