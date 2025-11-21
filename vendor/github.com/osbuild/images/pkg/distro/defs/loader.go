// package defs contain the distro definitions used by the "images" library
package defs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"text/template"

	"go.yaml.in/yaml/v3"

	"github.com/osbuild/images/data/distrodefs"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var (
	ErrNoPartitionTableForImgType = errors.New("no partition table for image type")
	ErrNoPartitionTableForArch    = errors.New("no partition table for arch")
)

// this can be overriden in tests
var defaultDataFS fs.FS = distrodefs.Data

func dataFS() fs.FS {
	// XXX: this is a short term measure, pass a set of
	// searchPaths down the stack instead
	dataFS := defaultDataFS
	if overrideDir := experimentalflags.String("yamldir"); overrideDir != "" {
		olog.Printf("WARNING: using experimental override dir %q", overrideDir)
		dataFS = os.DirFS(overrideDir)
	}
	return dataFS
}

// distrosYAML defines all supported YAML based distributions, since this can
// come from multiple sources we should make sure that we only have things in
// here that are easily merged
type distrosYAML struct {
	Distros []DistroYAML
}

type DistroYAML struct {
	// Match can be used to match/transform a given name
	// into the canonical <distro>-<major>{,.<minor>} form.
	// E.g.
	//   (?P<distro>rhel)-(?P<major>8)\.?(?P<minor>[0-9]+)
	// will support a format like e.g. rhel-810 and rhel-8.10
	Match string `yaml:"match"`

	// The distro metadata, can contain go text template strings
	// for {{.Major}}, {{.Minor}} which will be expanded by the
	// upper layers.
	Name             string            `yaml:"name"`
	Codename         string            `yaml:"codename"`
	Vendor           string            `yaml:"vendor"`
	Preview          bool              `yaml:"preview"`
	OsVersion        string            `yaml:"os_version"`
	ReleaseVersion   string            `yaml:"release_version"`
	ModulePlatformID string            `yaml:"module_platform_id"`
	Product          string            `yaml:"product"`
	OSTreeRefTmpl    string            `yaml:"ostree_ref_tmpl"`
	Runner           runner.RunnerConf `yaml:"runner"`

	// ISOLabelTmpl can contain {{.Product}},{{.OsVersion}},{{.Arch}},{{.ISOLabel}}
	ISOLabelTmpl string `yaml:"iso_label_tmpl"`

	DefaultFSType disk.FSType `yaml:"default_fs_type"`

	// directory with the actual image defintions, we separate that
	// so that we can point the "centos-10" distro to the "./rhel-10"
	// image types file/directory.
	DefsPath string `yaml:"defs_path"`

	BootstrapContainers map[arch.Arch]string `yaml:"bootstrap_containers"`

	OscapProfilesAllowList []oscap.Profile `yaml:"oscap_profiles_allowlist"`

	imageTypes map[string]ImageTypeYAML
	// distro wide default image config
	imageConfig *distro.ImageConfig `yaml:"default"`

	// ignore the given image types
	Conditions map[string]distroConditions `yaml:"conditions"`

	// XXX: remove this in favor of a better abstraction, this
	// is currently needed because the manifest pkg has conditionals
	// based on the distro, ideally it would not have this but
	// here we are.
	DistroLike manifest.Distro `yaml:"distro_like"`

	// set by the loader
	ID distro.ID
}

func (d *DistroYAML) ImageTypes() map[string]ImageTypeYAML {
	return d.imageTypes
}

// ImageConfig returns the distro wide default ImageConfig.
//
// Each ImageType gets this as their default ImageConfig.
func (d *DistroYAML) ImageConfig() *distro.ImageConfig {
	return d.imageConfig
}

func (d *DistroYAML) SkipImageType(imgTypeName, archName string) bool {
	for _, cond := range d.Conditions {
		if cond.When.Eval(d.ID, archName) && slices.Contains(cond.IgnoreImageTypes, imgTypeName) {
			return true
		}
	}

	return false
}

func (d *DistroYAML) runTemplates(id distro.ID) error {
	var errs []error
	subs := func(inp string) string {
		var buf bytes.Buffer
		templ, err := template.New("").Parse(inp)
		if err != nil {
			errs = append(errs, err)
			return inp
		}
		if err := templ.Execute(&buf, id); err != nil {
			errs = append(errs, err)
			return inp
		}
		return buf.String()
	}
	d.Name = subs(d.Name)
	d.OsVersion = subs(d.OsVersion)
	d.ReleaseVersion = subs(d.ReleaseVersion)
	d.OSTreeRefTmpl = subs(d.OSTreeRefTmpl)
	d.ModulePlatformID = subs(d.ModulePlatformID)
	d.Runner.Name = subs(d.Runner.Name)
	for a := range d.BootstrapContainers {
		d.BootstrapContainers[a] = subs(d.BootstrapContainers[a])
	}

	return errors.Join(errs...)
}

// Load all YAML files directly in the root of the definitions filesystem. Each
// file is read in sorted order and the entries found under the `distros` key
// are appended together.
// Note that files are read separately from each other, so anchors and other
// references can only be done within the same file.
func loadDistros() (*distrosYAML, error) {
	dents, err := fs.Glob(dataFS(), "*.yaml")
	if err != nil {
		return nil, err
	}

	var allDistros distrosYAML

	for _, name := range dents {
		f, err := dataFS().Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		decoder := yaml.NewDecoder(f)
		decoder.KnownFields(true)

		var distros distrosYAML
		if err := decoder.Decode(&distros); err != nil {
			return nil, err
		}

		allDistros.Distros = append(allDistros.Distros, distros.Distros...)
	}

	return &allDistros, nil
}

// NewDistroYAML return the given distro or nil if the distro is not
// found. This mimics the "distrofactory.GetDistro() interface.
//
// Note that eventually we want something like "Distros()" instead
// that returns all known distros but for now we keep compatibility
// with the way distrofactory/reporegistry work which is by defining
// distros via repository files.
func NewDistroYAML(nameVer string) (*DistroYAML, error) {
	foundDistro, err := LoadDistroWithoutImageTypes(nameVer)
	if err != nil {
		return nil, err
	}
	if foundDistro == nil {
		return nil, nil
	}

	if err := foundDistro.LoadImageTypes(); err != nil {
		return nil, err
	}
	return foundDistro, nil
}

func LoadDistroWithoutImageTypes(nameVer string) (*DistroYAML, error) {
	distros, err := loadDistros()
	if err != nil {
		return nil, err
	}

	var foundDistro *DistroYAML
	for _, distro := range distros.Distros {
		if distro.Name == nameVer {
			foundDistro = &distro
			break
		}

		found, err := matchAndNormalize(distro.Match, nameVer)
		if err != nil {
			return nil, err
		}
		if found != "" {
			foundDistro = &distro
			// nameVer must be replaced with normalized name
			nameVer = found
			break
		}
	}
	if foundDistro == nil {
		return nil, nil
	}

	// having "foundDistro.id" avoid re-parsing this in the various helpers
	id, err := distro.ParseID(nameVer)
	if err != nil {
		return nil, err
	}
	foundDistro.ID = *id
	if err := foundDistro.runTemplates(*id); err != nil {
		return nil, err
	}

	return foundDistro, err
}

func (d *DistroYAML) LoadImageTypes() error {
	f, err := dataFS().Open(filepath.Join(d.DefsPath, "imagetypes.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()

	var toplevel imageTypesYAML
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&toplevel); err != nil {
		return err
	}
	if len(toplevel.ImageTypes) > 0 {
		d.imageTypes = make(map[string]ImageTypeYAML, len(toplevel.ImageTypes))
		for name := range toplevel.ImageTypes {
			v := toplevel.ImageTypes[name]
			v.name = name
			if err := v.runTemplates(d); err != nil {
				return err
			}
			if err := v.setupDefaultFS(d.DefaultFSType.String()); err != nil {
				return err
			}

			d.imageTypes[name] = v
		}
	}
	d.imageConfig = toplevel.ImageConfig.For(d.ID)
	return nil
}

// imageTypesYAML describes the image types for a given distribution
// family. Note that multiple distros may use the same image types,
// e.g. centos/rhel
type imageTypesYAML struct {
	ImageConfig distroImageConfig        `yaml:"image_config,omitempty"`
	ImageTypes  map[string]ImageTypeYAML `yaml:"image_types"`
	Common      map[string]any           `yaml:".common,omitempty"`
}

type distroImageConfig struct {
	Default    *distro.ImageConfig                     `yaml:"default"`
	Conditions map[string]*distroImageConfigConditions `yaml:"conditions,omitempty"`
}

// multiple whenConditions are considred AND
type whenCondition struct {
	DistroName            string `yaml:"distro_name,omitempty"`
	NotDistroName         string `yaml:"not_distro_name,omitempty"`
	Architecture          string `yaml:"arch,omitempty"`
	VersionLessThan       string `yaml:"version_less_than,omitempty"`
	VersionGreaterOrEqual string `yaml:"version_greater_or_equal,omitempty"`
	VersionEqual          string `yaml:"version_equal,omitempty"`
}

func (wc *whenCondition) Eval(id distro.ID, archStr string) bool {
	match := true

	if wc.DistroName != "" {
		match = match && (wc.DistroName == id.Name)
	}
	if wc.NotDistroName != "" {
		match = match && (wc.NotDistroName != id.Name)
	}
	if wc.Architecture != "" {
		match = match && (wc.Architecture == archStr)
	}
	if wc.VersionLessThan != "" {
		match = match && (common.VersionLessThan(versionStringForVerCmp(id), wc.VersionLessThan))
	}
	if wc.VersionGreaterOrEqual != "" {
		match = match && (common.VersionGreaterThanOrEqual(versionStringForVerCmp(id), wc.VersionGreaterOrEqual))
	}
	if wc.VersionEqual != "" {
		match = match && (id.VersionString() == wc.VersionEqual)
	}

	return match
}

func (di *distroImageConfig) For(id distro.ID) *distro.ImageConfig {
	imgConfig := di.Default

	if di.Conditions != nil {
		for _, cond := range di.Conditions {
			// distro image config cannot have architecure
			// specific conditions
			arch := ""
			if cond.When.Eval(id, arch) {
				imgConfig = cond.ShallowMerge.InheritFrom(imgConfig)
			}
		}
	}

	return imgConfig
}

type distroImageConfigConditions struct {
	When         whenCondition       `yaml:"when,omitempty"`
	ShallowMerge *distro.ImageConfig `yaml:"shallow_merge,omitempty"`
}

type distroConditions struct {
	When             *whenCondition `yaml:"when"`
	IgnoreImageTypes []string       `yaml:"ignore_image_types"`
}

type ImageTypeYAML struct {
	// This maps "pkgsKey" to their package sets. The
	// map key here is a string that can either be:
	// - "os": packages for the os
	// - "installer": packages for the installer
	// - "container": extra package into an iot container
	//
	// - "blueprint": unused AFAICT
	// - "build": unused AFAICT
	// Note that this does not directly maps to pipeline names
	// but we should look into making it so.
	PackageSetsYAML map[string][]packageSet `yaml:"package_sets"`
	// archStr->partitionTable
	PartitionTables map[string]*disk.PartitionTable `yaml:"partition_table"`
	// override specific aspects of the partition table
	PartitionTablesOverrides *partitionTablesOverrides `yaml:"partition_tables_override"`

	ImageConfigYAML     imageConfig     `yaml:"image_config,omitempty"`
	InstallerConfigYAML installerConfig `yaml:"installer_config,omitempty"`

	Filename    string                      `yaml:"filename"`
	MimeType    string                      `yaml:"mime_type"`
	Compression string                      `yaml:"compression"`
	Environment environment.EnvironmentConf `yaml:"environment"`
	Bootable    bool                        `yaml:"bootable"`

	BootISO bool `yaml:"boot_iso"`
	// XXX merge with BootISO above, controls if grub2 or syslinux are used for ISO boots
	UseSyslinux             bool `yaml:"use_syslinux"`
	UseLegacyAnacondaConfig bool `yaml:"use_legacy_anaconda_config"`

	ISOLabel string `yaml:"iso_label"`
	// XXX: or iso_variant?
	Variant string `yaml:"variant"`

	RPMOSTree bool `yaml:"rpm_ostree"`

	OSTree struct {
		Name       string `yaml:"name"`
		RemoteName string `yaml:"remote_name"`
	} `yaml:"ostree"`
	// XXX: rhel-8 uses this
	UseOstreeRemotes bool `yaml:"use_ostree_remotes"`

	DefaultSize datasizes.Size `yaml:"default_size"`
	// the image func name: disk,container,live-installer,...
	Image                  string                    `yaml:"image_func"`
	Exports                []string                  `yaml:"exports"`
	RequiredPartitionSizes map[string]datasizes.Size `yaml:"required_partition_sizes"`

	InternalPlatforms []platform.Data    `yaml:"platforms"`
	PlatformsOverride *platformsOverride `yaml:"platforms_override"`

	NameAliases []string `yaml:"name_aliases"`

	InstallWeakDeps *bool `yaml:"install_weak_deps"`

	// for RHEL7 compat
	// TODO: determine a better place for these options, but for now they are here
	DiskImagePartTool     *osbuild.PartTool `yaml:"disk_image_part_tool"`
	DiskImageVPCForceSize *bool             `yaml:"disk_image_vpc_force_size"`

	SupportedPartitioningModes []partition.PartitioningMode `yaml:"supported_partitioning_modes"`

	SupportedBlueprintOptions []string `yaml:"supported_blueprint_options"`
	RequiredBlueprintOptions  []string `yaml:"required_blueprint_options"`

	// name is set by the loader
	name string
}

func (it *ImageTypeYAML) Name() string {
	return it.name
}

func (it *ImageTypeYAML) PlatformsFor(id distro.ID) ([]platform.Data, error) {
	pl := it.InternalPlatforms
	if it.PlatformsOverride != nil {
		var nMatches int
		for _, cond := range it.PlatformsOverride.Conditions {
			// arch does not make sense for platform overrides
			arch := ""
			if cond.When.Eval(id, arch) {
				pl = cond.Override
				nMatches++
			}
		}
		if nMatches > 1 {
			return nil, fmt.Errorf("platform conditionals for image type %q should match only once but matched %v times", it.Name(), nMatches)
		}
	}
	return pl, nil
}

func (it *ImageTypeYAML) runTemplates(distro *DistroYAML) error {
	var data any
	// set the DistroVendor in the struct only if its actually
	// set, this ensures that the template execution fails if the
	// template is used by the user has not set it
	if distro.Vendor != "" {
		data = struct {
			DistroVendor string
		}{
			DistroVendor: distro.Vendor,
		}
	}
	subs := func(inp string) (string, error) {
		templ, err := template.New("uefi-vendor").Parse(inp)
		templ.Option("missingkey=error")
		if err != nil {
			return "", fmt.Errorf(`cannot parse template for "vendor" field: %w`, err)
		}
		var buf bytes.Buffer
		if err := templ.Execute(&buf, data); err != nil {
			return "", fmt.Errorf(`cannot execute template for "vendor" field (is it set?): %w`, err)
		}
		return buf.String(), nil
	}
	for idx := range it.InternalPlatforms {
		newVendor, err := subs(it.InternalPlatforms[idx].UEFIVendor)
		if err != nil {
			return err
		}
		it.InternalPlatforms[idx].UEFIVendor = newVendor
	}
	if it.PlatformsOverride != nil {
		for _, cond := range it.PlatformsOverride.Conditions {
			for idx := range cond.Override {
				newVendor, err := subs(cond.Override[idx].UEFIVendor)
				if err != nil {
					return err
				}
				cond.Override[idx].UEFIVendor = newVendor
			}
		}
	}
	return nil
}

func (it *ImageTypeYAML) setupDefaultFS(distroDefaultFS string) error {
	subs := func(pts map[string]*disk.PartitionTable) error {
		for _, pt := range pts {
			err := pt.ForEachMountable(func(mnt disk.Mountable, _ []disk.Entity) error {
				elem, ok := mnt.(*disk.Filesystem)
				if !ok {
					return nil
				}
				if elem.Type == "" {
					if distroDefaultFS == "" {
						return fmt.Errorf("mount %q requires a default filesystem for the distribution but none set", mnt.GetMountpoint())
					}
					elem.Type = distroDefaultFS
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	// we need to update both the partition tables and all
	// partition tables overrides
	if err := subs(it.PartitionTables); err != nil {
		return err
	}
	if it.PartitionTablesOverrides != nil {
		for _, cond := range it.PartitionTablesOverrides.Conditions {
			if err := subs(cond.Override); err != nil {
				return err
			}
		}
	}

	return nil
}

type platformsOverride struct {
	Conditions map[string]*conditionsPlatforms `yaml:"conditions,omitempty"`
}

type conditionsPlatforms struct {
	When     whenCondition   `yaml:"when,omitempty"`
	Override []platform.Data `yaml:"override"`
}

type imageConfig struct {
	*distro.ImageConfig `yaml:",inline"`
	Conditions          map[string]*conditionsImgConf `yaml:"conditions,omitempty"`
}

type conditionsImgConf struct {
	When         whenCondition       `yaml:"when,omitempty"`
	ShallowMerge *distro.ImageConfig `yaml:"shallow_merge"`
}

type installerConfig struct {
	*distro.InstallerConfig `yaml:",inline"`
	Conditions              map[string]*conditionsInstallerConf `yaml:"conditions,omitempty"`
}

type conditionsInstallerConf struct {
	When         whenCondition           `yaml:"when,omitempty"`
	ShallowMerge *distro.InstallerConfig `yaml:"shallow_merge,omitempty"`
}

type packageSet struct {
	Include    []string                     `yaml:"include"`
	Exclude    []string                     `yaml:"exclude"`
	Conditions map[string]*pkgSetConditions `yaml:"conditions,omitempty"`
}

type pkgSetConditions struct {
	When   whenCondition `yaml:"when,omitempty"`
	Append struct {
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"append,omitempty"`
}

type partitionTablesOverrides struct {
	Conditions map[string]*partitionTablesOverwriteCondition `yaml:"conditions"`
}

type partitionTablesOverwriteCondition struct {
	When     whenCondition                   `yaml:"when,omitempty"`
	Override map[string]*disk.PartitionTable `yaml:"override"`
}

// versionStringForVerCmp is a special version string for our version
// compare that will assume that any version with no minor is
// automatically higher than any compare with a minor version.
//
// The rational is that "centos-9" is always higher than any "rhel-9.X"
// version for our version compare (centos is always "rolling").
//
// TODO: this should become an explicit chose in "imagetypes.yaml" but until
// we have everything converted to generic.Distro accessing the properites
// from an image type is very hard so we start here.
func versionStringForVerCmp(u distro.ID) string {
	if u.MinorVersion == -1 {
		u.MinorVersion = 999
	}
	return u.VersionString()
}

// PackageSets loads the PackageSets from the yaml source file
// discovered via the imagetype.
func (imgType *ImageTypeYAML) PackageSets(id distro.ID, archName string) map[string]rpmmd.PackageSet {
	res := make(map[string]rpmmd.PackageSet)
	for key, pkgSets := range imgType.PackageSetsYAML {
		var rpmmdPkgSet rpmmd.PackageSet
		for _, pkgSet := range pkgSets {
			rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
				Include: pkgSet.Include,
				Exclude: pkgSet.Exclude,
			})

			if pkgSet.Conditions != nil {
				for _, cond := range pkgSet.Conditions {
					if cond.When.Eval(id, archName) {
						rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
							Include: cond.Append.Include,
							Exclude: cond.Append.Exclude,
						})
					}
				}
			}
		}
		// mostly for tests
		sort.Strings(rpmmdPkgSet.Include)
		sort.Strings(rpmmdPkgSet.Exclude)
		res[key] = rpmmdPkgSet
	}

	return res
}

// PartitionTable returns the partionTable for the given distro/imgType.
func (imgType *ImageTypeYAML) PartitionTable(id distro.ID, archName string) (*disk.PartitionTable, error) {
	if imgType.PartitionTables == nil {
		return nil, fmt.Errorf("%w: %q", ErrNoPartitionTableForImgType, id)
	}
	pt, ok := imgType.PartitionTables[archName]
	if !ok {
		return nil, fmt.Errorf("%w (%q): %q", ErrNoPartitionTableForArch, id, archName)
	}

	if imgType.PartitionTablesOverrides != nil {
		for _, cond := range imgType.PartitionTablesOverrides.Conditions {
			if cond.When.Eval(id, archName) {
				pt = cond.Override[archName]
			}
		}
	}

	return pt, nil
}

// ImageConfig returns the image type specific ImageConfig
func (imgType *ImageTypeYAML) ImageConfig(id distro.ID, archName string) *distro.ImageConfig {
	imgConfig := imgType.ImageConfigYAML.ImageConfig
	for _, cond := range imgType.ImageConfigYAML.Conditions {
		if cond.When.Eval(id, archName) {
			imgConfig = cond.ShallowMerge.InheritFrom(imgConfig)

		}
	}

	return imgConfig
}

// InstallerConfig returns the InstallerConfig for the given imgType
// Note that on conditions the InstallerConfig is fully replaced, do
// any merging in YAML
func (imgType *ImageTypeYAML) InstallerConfig(id distro.ID, archName string) *distro.InstallerConfig {
	installerConfig := imgType.InstallerConfigYAML.InstallerConfig
	for _, cond := range imgType.InstallerConfigYAML.Conditions {
		if cond.When.Eval(id, archName) {
			installerConfig = cond.ShallowMerge.InheritFrom(installerConfig)
		}
	}

	return installerConfig
}
