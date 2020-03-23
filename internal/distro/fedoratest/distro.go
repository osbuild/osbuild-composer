package fedoratest

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const name = "fedora-30"
const modulePlatformID = "platform:f30"

type FedoraTestDistro struct{}

type fedoraTestDistroArch struct {
	name   string
	distro *FedoraTestDistro
}

type fedoraTestDistroImageType struct {
	name string
	arch *fedoraTestDistroArch
}

func (d *FedoraTestDistro) GetArch(arch string) (distro.Arch, error) {
	if arch != "x86_64" {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &fedoraTestDistroArch{
		name:   arch,
		distro: d,
	}, nil
}

func (a *fedoraTestDistroArch) Name() string {
	return a.name
}

func (a *fedoraTestDistroArch) ListImageTypes() []string {
	return []string{"qcow2"}
}

func (a *fedoraTestDistroArch) GetImageType(imageType string) (distro.ImageType, error) {
	if imageType != "qcow2" {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &fedoraTestDistroImageType{
		name: imageType,
		arch: a,
	}, nil
}

func (t *fedoraTestDistroImageType) Name() string {
	return t.name
}

func (t *fedoraTestDistroImageType) Filename() string {
	return "test.img"
}

func (t *fedoraTestDistroImageType) MIMEType() string {
	return "application/x-test"
}

func (t *fedoraTestDistroImageType) Size(size uint64) uint64 {
	return size
}

func (t *fedoraTestDistroImageType) BasePackages() ([]string, []string) {
	return nil, nil
}

func (t *fedoraTestDistroImageType) BuildPackages() []string {
	return nil
}

func (t *fedoraTestDistroImageType) Manifest(c *blueprint.Customizations,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec,
	size uint64) (*osbuild.Manifest, error) {
	return &osbuild.Manifest{
		Pipeline: osbuild.Pipeline{},
		Sources:  osbuild.Sources{},
	}, nil
}

func New() *FedoraTestDistro {
	return &FedoraTestDistro{}
}

func (d *FedoraTestDistro) Name() string {
	return name
}

func (d *FedoraTestDistro) ModulePlatformID() string {
	return modulePlatformID
}

func (d *FedoraTestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	if outputFormat == "qcow2" {
		return "test.img", "application/x-test", nil
	} else {
		return "", "", errors.New("invalid output format: " + outputFormat)
	}
}
