package fedoratest

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const ModulePlatformID = "platform:f30"

type FedoraTestDistro struct{}

type FedoraTestDistroArch struct {
	name   string
	distro *FedoraTestDistro
}

type FedoraTestDistroImageType struct {
	name string
	arch *FedoraTestDistroArch
}

func (d *FedoraTestDistro) GetArch(arch string) (distro.Arch, error) {
	if arch != "x86_64" {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &FedoraTestDistroArch{
		name:   arch,
		distro: d,
	}, nil
}

func (a *FedoraTestDistroArch) Name() string {
	return a.name
}

func (a *FedoraTestDistroArch) ListImageTypes() []string {
	return a.distro.ListOutputFormats()
}

func (a *FedoraTestDistroArch) GetImageType(imageType string) (distro.ImageType, error) {
	if imageType != "qcow2" {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &FedoraTestDistroImageType{
		name: imageType,
		arch: a,
	}, nil
}

func (t *FedoraTestDistroImageType) Name() string {
	return t.name
}

func (t *FedoraTestDistroImageType) Filename() string {
	return "test.img"
}

func (t *FedoraTestDistroImageType) MIMEType() string {
	return "application/x-test"
}

func (t *FedoraTestDistroImageType) Size(size uint64) uint64 {
	return t.arch.distro.GetSizeForOutputType(t.name, size)
}

func (t *FedoraTestDistroImageType) BasePackages() ([]string, []string) {
	return nil, nil
}

func (t *FedoraTestDistroImageType) BuildPackages() []string {
	return nil
}

func (t *FedoraTestDistroImageType) Manifest(c *blueprint.Customizations,
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
	return "fedora-30"
}

func (d *FedoraTestDistro) Distribution() common.Distribution {
	return common.Fedora30
}

func (d *FedoraTestDistro) ModulePlatformID() string {
	return ModulePlatformID
}

func (d *FedoraTestDistro) ListOutputFormats() []string {
	return []string{"qcow2"}
}

func (d *FedoraTestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	if outputFormat == "qcow2" {
		return "test.img", "application/x-test", nil
	} else {
		return "", "", errors.New("invalid output format: " + outputFormat)
	}
}

func (r *FedoraTestDistro) GetSizeForOutputType(outputFormat string, size uint64) uint64 {
	return 0
}

func (d *FedoraTestDistro) BasePackages(outputFormat string, outputArchitecture string) ([]string, []string, error) {
	return nil, nil, nil
}

func (d *FedoraTestDistro) BuildPackages(outputArchitecture string) ([]string, error) {
	return nil, nil
}

func (d *FedoraTestDistro) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, buildPackages, basePackages []rpmmd.PackageSpec, outputArch, outputFormat string, size uint64) (*osbuild.Pipeline, error) {
	if outputFormat == "qcow2" && outputArch == "x86_64" {
		return &osbuild.Pipeline{}, nil
	} else {
		return nil, errors.New("invalid output format or arch: " + outputFormat + " @ " + outputArch)
	}
}

func (r *FedoraTestDistro) sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
	return &osbuild.Sources{}
}

func (r *FedoraTestDistro) Manifest(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, outputFormat string, size uint64) (*osbuild.Manifest, error) {
	pipeline, err := r.pipeline(c, repos, packageSpecs, buildPackageSpecs, outputArchitecture, outputFormat, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *r.sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func (d *FedoraTestDistro) Runner() string {
	return "org.osbuild.test"
}
