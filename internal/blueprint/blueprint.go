// Package blueprint contains primitives for representing weldr blueprints and
// translating them to OSBuild pipelines
package blueprint

import (
	"sort"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

// An InvalidOutputFormatError is returned when a requested output format is
// not supported. The requested format is included as the error message.
type InvalidOutputFormatError struct {
	message string
}

func (e *InvalidOutputFormatError) Error() string {
	return e.message
}

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Version        string          `json:"version,omitempty"`
	Packages       []Package       `json:"packages"`
	Modules        []Package       `json:"modules"`
	Groups         []Group         `json:"groups"`
	Customizations *Customizations `json:"customizations,omitempty"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// A group specifies an package group.
type Group struct {
	Name string `json:"name"`
}

type output interface {
	translate(b *Blueprint) *pipeline.Pipeline
	getName() string
	getMime() string
}

var outputs = map[string]output{
	"ami":              &amiOutput{},
	"ext4-filesystem":  &ext4Output{},
	"live-iso":         &liveIsoOutput{},
	"partitioned-disk": &diskOutput{},
	"qcow2":            &qcow2Output{},
	"openstack":        &openstackOutput{},
	"tar":              &tarOutput{},
	"vhd":              &vhdOutput{},
	"vmdk":             &vmdkOutput{},
}

// ListOutputFormats returns a sorted list of the supported output formats
func ListOutputFormats() []string {
	formats := make([]string, 0, len(outputs))
	for name := range outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)

	return formats
}

// ToPipeline converts the blueprint to a pipeline for a given output format.
func (b *Blueprint) ToPipeline(outputFormat string) (*pipeline.Pipeline, error) {
	if output, exists := outputs[outputFormat]; exists {
		return output.translate(b), nil
	}

	return nil, &InvalidOutputFormatError{outputFormat}
}

func (b *Blueprint) getKernelCustomization() *KernelCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Kernel
}

// FilenameFromType gets the canonical filename and MIME type for a given
// output format
func FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := outputs[outputFormat]; exists {
		return output.getName(), output.getMime(), nil
	}

	return "", "", &InvalidOutputFormatError{outputFormat}
}
