package distro

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"
)

type Distro interface {
	// Returns a list of repositories from which this distribution gets its
	// content.
	Repositories() []rpmmd.RepoConfig

	// Returns a sorted list of the output formats this distro supports.
	ListOutputFormats() []string

	// Returns the canonical filename and MIME type for a given output
	// format. `outputFormat` must be one returned by
	FilenameFromType(outputFormat string) (string, string, error)

	// Returns an osbuild pipeline that generates an image in the given
	// output format with all packages and customizations specified in the
	// given blueprint.
	Pipeline(b *blueprint.Blueprint, checksums map[string]string, outputFormat string) (*pipeline.Pipeline, error)

	// Returns a osbuild runner that can be used on this distro.
	Runner() string
}

var registered map[string]Distro

func init() {
	registered = map[string]Distro{
		"fedora-30": fedora30.New(),
		"rhel-8.2":    rhel82.New(),
	}
}

func New(name string) Distro {
	distro, ok := registered[name]
	if !ok {
		return nil
	}

	return distro
}

func FromHost() (Distro, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	osrelease, err := readOSRelease(f)
	if err != nil {
		return nil, err
	}

	name := osrelease["ID"] + "-" + osrelease["VERSION_ID"]

	d := New(name)
	if d == nil {
		return nil, errors.New("unknown distro: " + name)
	}

	return d, nil
}

func Register(name string, distro Distro) {
	if _, exists := registered[name]; exists {
		panic("a distro with this name already exists: " + name)
	}
	registered[name] = distro
}

func readOSRelease(r io.Reader) (map[string]string, error) {
	osrelease := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("readOSRelease: invalid input")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value[0] == '"' {
			if len(value) < 2 || value[len(value) - 1] != '"' {
				return nil, errors.New("readOSRelease: invalid input")
			}
			value = value[1:len(value) - 1]
		}

		osrelease[key] = value
	}

	return osrelease, nil
}
