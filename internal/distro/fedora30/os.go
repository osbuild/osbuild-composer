package fedora30

import (
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Fedora30 struct {
	outputs map[string]output
}

type output interface {
	translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error)
	getName() string
	getMime() string
}

func init() {
	distro.Register("fedora-30", &Fedora30{
		outputs: map[string]output{
			"ami":              &amiOutput{},
			"ext4-filesystem":  &ext4Output{},
			"live-iso":         &liveIsoOutput{},
			"partitioned-disk": &diskOutput{},
			"qcow2":            &qcow2Output{},
			"openstack":        &openstackOutput{},
			"tar":              &tarOutput{},
			"vhd":              &vhdOutput{},
			"vmdk":             &vmdkOutput{},
		},
	})
}

func (f *Fedora30) Repositories() []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:       "fedora",
			Name:     "Fedora 30",
			Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
		},
	}
}

// ListOutputFormats returns a sorted list of the supported output formats
func (f *Fedora30) ListOutputFormats() []string {
	formats := make([]string, 0, len(f.outputs))
	for name := range f.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (f *Fedora30) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := f.outputs[outputFormat]; exists {
		return output.getName(), output.getMime(), nil
	}
	return "", "", &distro.InvalidOutputFormatError{outputFormat}
}

func (f *Fedora30) Pipeline(b *blueprint.Blueprint, outputFormat string) (*pipeline.Pipeline, error) {
	if output, exists := f.outputs[outputFormat]; exists {
		return output.translate(b)
	}
	return nil, &distro.InvalidOutputFormatError{outputFormat}
}
