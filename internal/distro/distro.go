package distro

import (
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
	distro, ok := registered[name]
	if !ok {
		panic("unknown distro: " + name)
	}
	return distro
}

func Register(name string, distro Distro) {
	if _, exists := registered[name]; exists {
		panic("a distro with this name already exists: " + name)
	}
	registered[name] = distro
}
