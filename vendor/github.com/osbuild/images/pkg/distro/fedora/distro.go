package fedora

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

const (
	// package set names

	// main/common os image package set name
	osPkgsKey = "os"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"
)

var (
	oscapProfileAllowList = []oscap.Profile{
		oscap.Ospp,
		oscap.PciDss,
		oscap.Standard,
	}
)

type distribution struct {
	name               string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	ostreeRefTmpl      string
	runner             runner.Runner
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

func getISOLabelFunc(variant string) isoLabelFunc {
	const ISO_LABEL = "%s-%s-%s-%s"

	return func(t *imageType) string {
		return fmt.Sprintf(ISO_LABEL, t.Arch().Distro().Product(), t.Arch().Distro().OsVersion(), variant, t.Arch().Name())
	}

}

func getDistro(version int) distribution {
	if version < 0 {
		panic("Invalid Fedora version (must be positive)")
	}
	nameVer := fmt.Sprintf("fedora-%d", version)
	return distribution{
		name:               nameVer,
		product:            "Fedora",
		osVersion:          strconv.Itoa(version),
		releaseVersion:     strconv.Itoa(version),
		modulePlatformID:   fmt.Sprintf("platform:f%d", version),
		ostreeRefTmpl:      fmt.Sprintf("fedora/%d/%%s/iot", version),
		runner:             &runner.Fedora{Version: uint64(version)},
		defaultImageConfig: common.Must(defs.DistroImageConfig(nameVer)),
	}
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Codename() string {
	return "" // Fedora does not use distro codename
}

func (d *distribution) Releasever() string {
	return d.releaseVersion
}

func (d *distribution) OsVersion() string {
	return d.releaseVersion
}

func (d *distribution) Product() string {
	return d.product
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRefTmpl
}

func (d *distribution) ListArches() []string {
	archNames := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archNames = append(archNames, name)
	}
	sort.Strings(archNames)
	return archNames
}

func (d *distribution) GetArch(name string) (distro.Arch, error) {
	arch, exists := d.arches[name]
	if !exists {
		return nil, errors.New("invalid architecture: " + name)
	}
	return arch, nil
}

func (d *distribution) addArches(arches ...architecture) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	// Do not make copies of architectures, as opposed to image types,
	// because architecture definitions are not used by more than a single
	// distro definition.
	for idx := range arches {
		d.arches[arches[idx].name] = &arches[idx]
	}
}

func (d *distribution) getDefaultImageConfig() *distro.ImageConfig {
	return d.defaultImageConfig
}

type architecture struct {
	distro           *distribution
	name             string
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
}

func (a *architecture) Name() string {
	return a.name
}

func (a *architecture) ListImageTypes() []string {
	itNames := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		itNames = append(itNames, name)
	}
	sort.Strings(itNames)
	return itNames
}

func (a *architecture) GetImageType(name string) (distro.ImageType, error) {
	t, exists := a.imageTypes[name]
	if !exists {
		aliasForName, exists := a.imageTypeAliases[name]
		if !exists {
			return nil, errors.New("invalid image type: " + name)
		}
		t, exists = a.imageTypes[aliasForName]
		if !exists {
			panic(fmt.Sprintf("image type '%s' is an alias to a non-existing image type '%s'", name, aliasForName))
		}
	}
	return t, nil
}

func (a *architecture) addImageTypes(platform platform.Platform, imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		it.platform = platform
		a.imageTypes[it.name] = &it
		for _, alias := range it.nameAliases {
			if a.imageTypeAliases == nil {
				a.imageTypeAliases = map[string]string{}
			}
			if existingAliasFor, exists := a.imageTypeAliases[alias]; exists {
				panic(fmt.Sprintf("image type alias '%s' for '%s' is already defined for another image type '%s'", alias, it.name, existingAliasFor))
			}
			a.imageTypeAliases[alias] = it.name
		}
	}
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

func newDistro(version int) distro.Distro {
	rd := getDistro(version)

	// XXX: generate architecture automatically from the imgType yaml
	x86_64 := architecture{
		name:   arch.ARCH_X86_64.String(),
		distro: &rd,
	}

	aarch64 := architecture{
		name:   arch.ARCH_AARCH64.String(),
		distro: &rd,
	}

	ppc64le := architecture{
		distro: &rd,
		name:   arch.ARCH_PPC64LE.String(),
	}

	s390x := architecture{
		distro: &rd,
		name:   arch.ARCH_S390X.String(),
	}

	riscv64 := architecture{
		name:   arch.ARCH_RISCV64.String(),
		distro: &rd,
	}

	// XXX: move all image types should to YAML
	its, err := defs.ImageTypes(rd.name)
	if err != nil {
		panic(err)
	}
	for _, imgTypeYAML := range its {
		// use as marker for images that are not converted to
		// YAML yet
		if imgTypeYAML.Filename == "" {
			continue
		}
		it := newImageTypeFrom(rd, imgTypeYAML)
		for _, pl := range imgTypeYAML.Platforms {
			switch pl.Arch {
			case arch.ARCH_X86_64:
				x86_64.addImageTypes(&pl, it)
			case arch.ARCH_AARCH64:
				aarch64.addImageTypes(&pl, it)
			case arch.ARCH_PPC64LE:
				ppc64le.addImageTypes(&pl, it)
			case arch.ARCH_S390X:
				s390x.addImageTypes(&pl, it)
			case arch.ARCH_RISCV64:
				riscv64.addImageTypes(&pl, it)
			default:
				err := fmt.Errorf("unsupported arch: %v", pl.Arch)
				panic(err)
			}
		}
	}

	rd.addArches(x86_64, aarch64, ppc64le, s390x, riscv64)
	return &rd
}

func ParseID(idStr string) (*distro.ID, error) {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil, err
	}

	if id.Name != "fedora" {
		return nil, fmt.Errorf("invalid distro name: %s", id.Name)
	}

	if id.MinorVersion != -1 {
		return nil, fmt.Errorf("fedora distro does not support minor versions")
	}

	return id, nil
}

func DistroFactory(idStr string) distro.Distro {
	id, err := ParseID(idStr)
	if err != nil {
		return nil
	}

	return newDistro(id.MajorVersion)
}
