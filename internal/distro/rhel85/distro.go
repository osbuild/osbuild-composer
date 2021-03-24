package rhel85

import (
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const name = "rhel-85"
const modulePlatformID = "platform:el8"
const ostreeRef = "rhel/8/%s/edge"

type distribution struct {
	arches map[string]distro.Arch
}

func (d *distribution) Name() string {
	return name
}

func (d *distribution) ModulePlatformID() string {
	return modulePlatformID
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

	for _, a := range arches {
		d.arches[a.name] = &architecture{
			distro:     d,
			name:       a.name,
			imageTypes: a.imageTypes,
		}
	}
}

type architecture struct {
	distro     *distribution
	name       string
	imageTypes map[string]distro.ImageType
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
		return nil, errors.New("invalid image type: " + name)
	}
	return t, nil
}

func (a *architecture) addImageTypes(imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for _, it := range imageTypes {
		a.imageTypes[it.name] = &imageType{
			arch:             a,
			name:             it.name,
			filename:         it.filename,
			mimeType:         it.mimeType,
			packageSets:      it.packageSets,
			enabledServices:  it.enabledServices,
			disabledServices: it.disabledServices,
			defaultTarget:    it.defaultTarget,
			bootISO:          it.bootISO,
			rpmOstree:        it.rpmOstree,
			defaultSize:      it.defaultSize,
			exports:          it.exports,
		}
	}
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

type imageType struct {
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]rpmmd.PackageSet
	enabledServices  []string
	disabledServices []string
	defaultTarget    string
	bootISO          bool
	rpmOstree        bool
	defaultSize      uint64
	exports          []string
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	if t.rpmOstree {
		return fmt.Sprintf(ostreeRef, t.arch.name)
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	return nil
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *imageType) Manifest(b *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSpecSets map[string][]rpmmd.PackageSpec, seed int64) (distro.Manifest, error) {
	return nil, nil
}

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	rd := new(distribution)

	x86_64 := architecture{
		name:   "x86_64",
		distro: rd,
	}
	aarch64 := architecture{
		name:   "aarch64",
		distro: rd,
	}
	ppc64le := architecture{
		distro: rd,
		name:   "ppc64le",
	}
	s390x := architecture{
		distro: rd,
		name:   "s390x",
	}
	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return rd
}
