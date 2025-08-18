package generic

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"text/template"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/platform"
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
	ErrDistroNotFound = errors.New("distribution not found")
)

// distribution implements the distro.Distro interface
var _ = distro.Distro(&distribution{})

type distribution struct {
	defs.DistroYAML

	arches map[string]*architecture
}

func (d *distribution) getISOLabelFunc(isoLabel string) isoLabelFunc {
	id := common.Must(distro.ParseID(d.Name()))

	return func(t *imageType) string {
		type inputs struct {
			Distro   *distro.ID
			Product  string
			Arch     string
			ISOLabel string
		}
		templ := common.Must(template.New("iso-label").Parse(d.DistroYAML.ISOLabelTmpl))
		var buf bytes.Buffer
		err := templ.Execute(&buf, inputs{
			Distro:   id,
			Product:  d.Product(),
			Arch:     t.Arch().Name(),
			ISOLabel: isoLabel,
		})
		if err != nil {
			// XXX: cleanup isoLabelFunc to allow error
			panic(err)
		}
		return buf.String()
	}
}

func newDistro(nameVer string) (distro.Distro, error) {
	distroYAML, err := defs.NewDistroYAML(nameVer)
	if err != nil {
		return nil, err
	}
	if distroYAML == nil {
		return nil, nil
	}

	rd := &distribution{
		DistroYAML: *distroYAML,
		arches:     make(map[string]*architecture),
	}

	for _, imgTypeYAML := range distroYAML.ImageTypes() {
		// use as marker for images that are not converted to
		// YAML yet
		if imgTypeYAML.Filename == "" {
			continue
		}
		platforms, err := imgTypeYAML.PlatformsFor(distroYAML.ID)
		if err != nil {
			return nil, err
		}
		for _, pl := range platforms {
			ar, ok := rd.arches[pl.Arch.String()]
			if !ok {
				ar = newArchitecture(rd, pl.Arch)
				rd.arches[pl.Arch.String()] = ar
			}
			if distroYAML.SkipImageType(imgTypeYAML.Name(), pl.Arch.String()) {
				continue
			}
			it := newImageTypeFrom(rd, ar, imgTypeYAML)
			if err := ar.addImageType(&pl, it); err != nil {
				return nil, err
			}
		}
	}

	return rd, nil
}

func (d *distribution) Name() string {
	return d.DistroYAML.Name
}

func (d *distribution) Codename() string {
	return d.DistroYAML.Codename
}

func (d *distribution) Releasever() string {
	return d.DistroYAML.ReleaseVersion
}

func (d *distribution) OsVersion() string {
	return d.DistroYAML.OsVersion
}

func (d *distribution) Product() string {
	return d.DistroYAML.Product
}

func (d *distribution) ModulePlatformID() string {
	return d.DistroYAML.ModulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.DistroYAML.OSTreeRefTmpl
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
		return nil, fmt.Errorf("invalid architecture: %v", name)
	}
	return arch, nil
}

// architecture implements the distro.Arch interface
var _ = distro.Arch(&architecture{})

type architecture struct {
	distro           *distribution
	arch             arch.Arch
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
}

func newArchitecture(rd *distribution, arch arch.Arch) *architecture {
	return &architecture{
		distro:           rd,
		arch:             arch,
		imageTypes:       make(map[string]distro.ImageType),
		imageTypeAliases: make(map[string]string),
	}
}

func (a *architecture) Name() string {
	return a.arch.String()
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
			return nil, fmt.Errorf("invalid image type: %v", name)
		}
		t, exists = a.imageTypes[aliasForName]
		if !exists {
			panic(fmt.Sprintf("image type '%s' is an alias to a non-existing image type '%s'", name, aliasForName))
		}
	}
	return t, nil
}

func (a *architecture) addImageType(platform platform.Platform, it imageType) error {
	it.arch = a
	it.platform = platform
	a.imageTypes[it.Name()] = &it
	for _, alias := range it.ImageTypeYAML.NameAliases {
		if a.imageTypeAliases == nil {
			a.imageTypeAliases = map[string]string{}
		}
		if existingAliasFor, exists := a.imageTypeAliases[alias]; exists {
			return fmt.Errorf("image type alias '%s' for '%s' is already defined for another image type '%s'", alias, it.Name(), existingAliasFor)
		}
		a.imageTypeAliases[alias] = it.Name()
	}
	return nil
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

func DistroFactory(idStr string) distro.Distro {
	distro, err := newDistro(idStr)
	if errors.Is(err, ErrDistroNotFound) {
		return nil
	}
	if err != nil {
		panic(fmt.Errorf("%w with distro %s", err, idStr))
	}
	return distro
}
