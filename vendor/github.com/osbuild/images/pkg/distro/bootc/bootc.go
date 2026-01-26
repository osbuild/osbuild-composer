package bootc

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	bibcontainer "github.com/osbuild/images/pkg/bib/container"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
)

var _ = distro.CustomDepsolverDistro(&BootcDistro{})

type BootcDistro struct {
	imgref          string
	buildImgref     string
	sourceInfo      *osinfo.Info
	buildSourceInfo *osinfo.Info

	id            distro.ID
	defaultFs     string
	releasever    string
	rootfsMinSize uint64

	arches map[string]distro.Arch
}

var _ = distro.Arch(&Arch{})

type Arch struct {
	distro *BootcDistro
	arch   arch.Arch

	imageTypes map[string]distro.ImageType
}

func (d *BootcDistro) SetBuildContainer(imgref string) (err error) {
	if imgref == "" {
		return nil
	}

	cnt, err := bibcontainer.New(imgref)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cnt.Stop())
	}()

	info, err := osinfo.Load(cnt.Root())
	if err != nil {
		return err
	}
	return d.setBuildContainer(imgref, info)
}

func (d *BootcDistro) setBuildContainer(imgref string, info *osinfo.Info) error {
	d.buildImgref = imgref
	d.buildSourceInfo = info
	return nil
}

// SetBuildContainerForTesting should only be used for in tests
// please use "SetBuildContainer" instead
func (d *BootcDistro) SetBuildContainerForTesting(imgref string, info *osinfo.Info) error {
	return d.setBuildContainer(imgref, info)
}

func (d *BootcDistro) DefaultFs() string {
	return d.defaultFs
}

func (d *BootcDistro) Name() string {
	return d.id.String()
}

func (d *BootcDistro) Codename() string {
	return ""
}

func (d *BootcDistro) Releasever() string {
	return d.releasever
}

func (d *BootcDistro) OsVersion() string {
	return d.releasever
}

func (d *BootcDistro) Product() string {
	return d.id.String()
}

func (d *BootcDistro) ModulePlatformID() string {
	return ""
}

func (d *BootcDistro) OSTreeRef() string {
	return ""
}

func (d *BootcDistro) Depsolver(rpmCacheRoot string, archi arch.Arch) (solver *depsolvednf.Solver, cleanup func() error, err error) {
	cnt, err := bibcontainer.New(d.buildImgref)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, cnt.Stop())
		}
	}()

	cleanup = func() error {
		return cnt.Stop()
	}
	if err := cnt.InitDNF(); err != nil {
		// Not all bootc container have dnf, so check if it can
		// be run here and if not just return nil which will
		// ensure the depsolver of the host is used
		if errors.Is(err, bibcontainer.ErrNoDnf) {
			return nil, cleanup, nil
		}
		// Return any other errors to the caller, it means
		// dnf is installed but not working.
		return nil, nil, err
	}
	solver, err = cnt.NewContainerSolver(rpmCacheRoot, archi, d.buildSourceInfo)
	if err != nil {
		return nil, nil, err
	}

	return solver, cleanup, nil
}

func (d *BootcDistro) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *BootcDistro) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, fmt.Errorf("requested bootc arch %q does not match available arches %v", arch, slices.Collect(maps.Keys(d.arches)))
	}
	return a, nil
}

func (d *BootcDistro) addArches(arches ...*Arch) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	for _, a := range arches {
		a.distro = d
		d.arches[a.Name()] = a
	}
}

func (a *Arch) Name() string {
	return a.arch.String()
}

func (a *Arch) Distro() distro.Distro {
	return a.distro
}

func (a *Arch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *Arch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return t, nil
}

func (a *Arch) addImageTypes(imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		a.imageTypes[it.Name()] = &it
	}
}

func newBootcDistroAfterIntrospect(archStr string, info *osinfo.Info, imgref, defaultFsStr string, cntSize uint64) (*BootcDistro, error) {
	nameVer := fmt.Sprintf("bootc-%s-%s", info.OSRelease.ID, info.OSRelease.VersionID)
	id, err := distro.ParseID(nameVer)
	if err != nil {
		return nil, err
	}
	bd := &BootcDistro{
		id:            *id,
		releasever:    info.OSRelease.VersionID,
		defaultFs:     defaultFsStr,
		rootfsMinSize: cntSize * containerSizeToDiskSizeMultiplier,

		imgref:     imgref,
		sourceInfo: info,
		// default buildref/info to regular container, this can
		// be overriden with SetBuildContainer()
		buildImgref:     imgref,
		buildSourceInfo: info,
	}

	archi, err := arch.FromString(archStr)
	if err != nil {
		return nil, err
	}
	ba := &Arch{
		arch: archi,
	}

	distroYAML, err := defs.LoadDistroWithoutImageTypes("bootc-generic-1")
	if err != nil {
		return nil, err
	}
	defaultFs, err := disk.NewFSType(defaultFsStr)
	if err != nil {
		return nil, err
	}
	distroYAML.DefaultFSType = defaultFs
	if err := distroYAML.LoadImageTypes(); err != nil {
		return nil, err
	}
	for _, imgTypeYaml := range distroYAML.ImageTypes() {
		ba.addImageTypes(imageType{
			ImageTypeYAML: imgTypeYaml,
		})
	}
	bd.addArches(ba)

	return bd, nil
}

// NewBootcDistroForTesting can be used to generate test manifests.
// The container introspection is skipped. Do not use this for
// anything but tests.
var NewBootcDistroForTesting = newBootcDistroAfterIntrospect

func DistroFactory(idStr string) distro.Distro {
	l := strings.SplitN(idStr, ":", 2)
	if l[0] != "bootc" {
		return nil
	}
	imgRef := l[1]

	return common.Must(NewBootcDistro(imgRef, nil))
}
