package fedoratest

import (
	"encoding/json"
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const name = "fedora-30"
const modulePlatformID = "platform:f30"

type FedoraTestDistro struct{}

type arch struct {
	name   string
	distro *FedoraTestDistro
}

type imageType struct {
	name string
	arch *arch
}

func (a *arch) Distro() distro.Distro {
	return a.distro
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (d *FedoraTestDistro) ListArches() []string {
	return []string{"x86_64"}
}

func (d *FedoraTestDistro) GetArch(name string) (distro.Arch, error) {
	if name != "x86_64" {
		return nil, errors.New("invalid architecture: " + name)
	}

	return &arch{
		name:   name,
		distro: d,
	}, nil
}

func (a *arch) Name() string {
	return a.name
}

func (a *arch) ListImageTypes() []string {
	return []string{"qcow2"}
}

func (a *arch) GetImageType(name string) (distro.ImageType, error) {
	if name != "qcow2" {
		return nil, errors.New("invalid image type: " + name)
	}

	return &imageType{
		name: name,
		arch: a,
	}, nil
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Filename() string {
	return "test.img"
}

func (t *imageType) MIMEType() string {
	return "application/x-test"
}

func (t *imageType) OSTreeRef() string {
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	return size
}

func (t *imageType) Packages(bp blueprint.Blueprint) ([]string, []string) {
	return nil, nil
}

func (t *imageType) BuildPackages() []string {
	return nil
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {

	return json.Marshal(
		osbuild.Manifest{
			Sources:  osbuild.Sources{},
			Pipeline: osbuild.Pipeline{},
		},
	)
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
