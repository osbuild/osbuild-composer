package distro

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type Distro interface {
	// Returns a sorted list of the output formats this distro supports.
	ListOutputFormats() []string

	// Returns the canonical filename and MIME type for a given output
	// format
	FilenameFromType(outputFormat string) (string, string, error)

	// Returns an osbuild pipeline that generates an image in the given
	// output format with all packages and customizations specified in the
	// given blueprint. `outputFormat` must be one returned by
	// ListOutputFormats().
	Pipeline(b *blueprint.Blueprint, outputFormat string) (*pipeline.Pipeline, error)
}

// An InvalidOutputFormatError is returned when a requested output format is
// not supported. The requested format is included as the error message.
type InvalidOutputFormatError struct {
	Format string
}

func (e *InvalidOutputFormatError) Error() string {
	return e.Format
}

var registered = map[string]Distro{}

func New(name string) Distro {
	if name == "" {
		distro, err := FromHost()
		if err == nil {
			return distro
		} else {
			log.Println("cannot detect distro from host: " + err.Error())
			log.Println("falling back to 'fedora-30'")
			return New("fedora-30")
		}
	}

	distro, ok := registered[name]
	if !ok {
		panic("unknown distro: " + name)
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

	distro, ok := registered[name]
	if !ok {
		return nil, errors.New("unknown distro: " + name)
	}
	return distro, nil
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
