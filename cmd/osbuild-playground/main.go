package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// osbuild-playground is a utility command and is often run from within the
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

func main() {
	// Path to MyOptions or '-' for stdin
	myOptionsArg := flag.Arg(0)

	myOptions := &MyOptions{}
	if myOptionsArg != "" {
		var reader io.Reader
		if myOptionsArg == "-" {
			reader = os.Stdin
		} else {
			var err error
			reader, err = os.Open(myOptionsArg)
			if err != nil {
				panic("Could not open path to image options: " + err.Error())
			}
		}
		file, err := ioutil.ReadAll(reader)
		if err != nil {
			panic("Could not read image options: " + err.Error())
		}
		err = json.Unmarshal(file, &myOptions)
		if err != nil {
			panic("Could not parse image options: " + err.Error())
		}
	}

	distros := distroregistry.NewDefault()
	d := distros.FromHost()
	if d == nil {
		panic("host distro not supported")
	}

	arch, err := d.GetArch(common.CurrentArch())
	if err != nil {
		panic("host arch not supported")
	}

	repos, err := rpmmd.LoadRepositories([]string{"./"}, d.Name())
	if err != nil {
		panic("could not load repositories for distro " + d.Name())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch.Name(), path.Join(home, ".cache/osbuild-playground/rpmmd"))
	solver.SetDNFJSONPath(findDnfJsonBin())

	// Set cache size to 3 GiB
	solver.SetMaxCacheSize(1 * 1024 * 1024 * 1024)

	manifest := manifest.New()

	// TODO: figure out the runner situation
	err = MyManifest(&manifest, myOptions, repos[arch.Name()], "org.osbuild.fedora36")
	if err != nil {
		panic("MyManifest() failed: " + err.Error())
	}

	packageSpecs := make(map[string][]rpmmd.PackageSpec)
	for name, chain := range manifest.GetPackageSetChains() {
		packages, err := solver.Depsolve(chain)
		if err != nil {
			panic(fmt.Sprintf("failed to depsolve for pipeline %s: %s\n", name, err.Error()))
		}
		packageSpecs[name] = packages
	}

	bytes, err := manifest.Serialize(packageSpecs)
	if err != nil {
		panic("failed to serialize manifest: " + err.Error())
	}

	os.Stdout.Write(bytes)
	if err := solver.CleanCache(); err != nil {
		// print to stderr but don't exit with error
		fmt.Fprintf(os.Stderr, "could not clean dnf cache: %s", err.Error())
	}
}
