// package defs contain the distro definitions used by the "images" library
package defs

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

var (
	ErrImageTypeNotFound          = errors.New("image type not found")
	ErrNoPartitionTableForImgType = errors.New("no partition table for image type")
	ErrNoPartitionTableForArch    = errors.New("no partition table for arch")
)

//go:embed */*.yaml
var data embed.FS

var DataFS fs.FS = data

type toplevelYAML struct {
	ImageConfig distroImageConfig    `yaml:"image_config,omitempty"`
	ImageTypes  map[string]imageType `yaml:"image_types"`
	Common      map[string]any       `yaml:".common,omitempty"`
}

type distroImageConfig struct {
	Default   *distro.ImageConfig          `yaml:"default"`
	Condition *distroImageConfigConditions `yaml:"condition,omitempty"`
}

type distroImageConfigConditions struct {
	DistroName map[string]*distro.ImageConfig `yaml:"distro_name,omitempty"`
}

// XXX: this should eventually implement the "distro.ImageType"
// interface, then we don't need to convert into a fedora/rhel
// imagetype anymore (those will go away in subsequent refactors)
type ImageTypeYAML = imageType

type imageType struct {
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
	PackageSets map[string][]packageSet `yaml:"package_sets"`
	// archStr->partitionTable
	PartitionTables map[string]*disk.PartitionTable `yaml:"partition_table"`
	// override specific aspects of the partition table
	PartitionTablesOverrides *partitionTablesOverrides `yaml:"partition_tables_override"`

	ImageConfig     imageConfig     `yaml:"image_config,omitempty"`
	InstallerConfig installerConfig `yaml:"installer_config,omitempty"`

	Filename    string                      `yaml:"filename"`
	MimeType    string                      `yaml:"mime_type"`
	Compression string                      `yaml:"compression"`
	Environment environment.EnvironmentConf `yaml:"environment"`
	Bootable    bool                        `yaml:"bootable"`

	BootISO   bool   `yaml:"boot_iso"`
	ISOLabel  string `yaml:"iso_label"`
	RPMOSTree bool   `yaml:"rpm_ostree"`

	DefaultSize uint64 `yaml:"default_size"`
	// the image func name: disk,container,live-installer,...
	Image                  string            `yaml:"image_func"`
	BuildPipelines         []string          `yaml:"build_pipelines"`
	PayloadPipelines       []string          `yaml:"payload_pipelines"`
	Exports                []string          `yaml:"exports"`
	RequiredPartitionSizes map[string]uint64 `yaml:"required_partition_sizes"`

	Platforms []platform.PlatformConf `yaml:"platforms"`

	NameAliases []string `yaml:"name_aliases"`

	// name is set by the loader
	name string
}

func (it *imageType) Name() string {
	return it.name
}

type imageConfig struct {
	*distro.ImageConfig `yaml:",inline"`
	Condition           *conditionsImgConf `yaml:"condition,omitempty"`
}

type conditionsImgConf struct {
	Architecture    map[string]*distro.ImageConfig `yaml:"architecture,omitempty"`
	DistroName      map[string]*distro.ImageConfig `yaml:"distro_name,omitempty"`
	VersionLessThan map[string]*distro.ImageConfig `yaml:"version_less_than,omitempty"`
}

type installerConfig struct {
	*distro.InstallerConfig `yaml:",inline"`
	Condition               *conditionsInstallerConf `yaml:"condition,omitempty"`
}

type conditionsInstallerConf struct {
	Architecture    map[string]*distro.InstallerConfig `yaml:"architecture,omitempty"`
	DistroName      map[string]*distro.InstallerConfig `yaml:"distro_name,omitempty"`
	VersionLessThan map[string]*distro.InstallerConfig `yaml:"version_less_than,omitempty"`
}

type packageSet struct {
	Include   []string          `yaml:"include"`
	Exclude   []string          `yaml:"exclude"`
	Condition *pkgSetConditions `yaml:"condition,omitempty"`
}

type pkgSetConditions struct {
	Architecture          map[string]packageSet `yaml:"architecture,omitempty"`
	VersionLessThan       map[string]packageSet `yaml:"version_less_than,omitempty"`
	VersionGreaterOrEqual map[string]packageSet `yaml:"version_greater_or_equal,omitempty"`
	DistroName            map[string]packageSet `yaml:"distro_name,omitempty"`
}

type partitionTablesOverrides struct {
	Condition *partitionTablesOverwriteCondition `yaml:"condition"`
}

type partitionTablesOverwriteCondition struct {
	DistroName            map[string]map[string]*disk.PartitionTable `yaml:"distro_name,omitempty"`
	VersionGreaterOrEqual map[string]map[string]*disk.PartitionTable `yaml:"version_greater_or_equal,omitempty"`
	VersionLessThan       map[string]map[string]*disk.PartitionTable `yaml:"version_less_than,omitempty"`
}

// XXX: use slices.Backward() once we move to go1.23
// hint: use "git blame" on this comment and just revert
// the commit that adds it and you will have the 1.23 version
func backward[Slice ~[]E, E any](s Slice) []E {
	out := make([]E, 0, len(s))
	for i := len(s) - 1; i >= 0; i-- {
		out = append(out, s[i])
	}
	return out
}

// XXX: use slices.SortedFunc() once we move to go1.23
// hint: use "git blame" on this comment and just revert
// the commit that adds it and you will have the 1.23 version
func versionLessThanSortedKeys[T any](m map[string]T) []string {
	versions := maps.Keys(m)
	slices.SortFunc(versions, func(a, b string) int {
		ver1 := version.Must(version.NewVersion(a))
		ver2 := version.Must(version.NewVersion(b))
		switch {
		case ver1 == ver2:
			return 0
		case ver2.LessThan(ver1):
			return -1
		default:
			return 1
		}
	})
	return versions
}

// DistroImageConfig returns the distro wide ImageConfig.
//
// Each ImageType gets this as their default ImageConfig.
func DistroImageConfig(distroNameVer string) (*distro.ImageConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgConfig := toplevel.ImageConfig.Default

	cond := toplevel.ImageConfig.Condition
	if cond != nil {
		distroName, _ := splitDistroNameVer(distroNameVer)
		// XXX: we shoudl probably use a similar pattern like
		// for the partition table overrides (via
		// findElementIndexByJSONTag) but this if fine for now
		if distroNameCnf, ok := cond.DistroName[distroName]; ok {
			imgConfig = distroNameCnf.InheritFrom(imgConfig)
		}
	}

	return imgConfig, nil
}

// PackageSets loads the PackageSets from the yaml source file
// discovered via the imagetype.
func PackageSets(it distro.ImageType, replacements map[string]string) (map[string]rpmmd.PackageSet, error) {
	typeName := it.Name()

	arch := it.Arch()
	archName := arch.Name()
	distribution := arch.Distro()
	distroNameVer := distribution.Name()
	distroName, distroVersion := splitDistroNameVer(distroNameVer)

	// each imagetype can have multiple package sets, so that we can
	// use yaml aliases/anchors to de-duplicate them
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}

	res := make(map[string]rpmmd.PackageSet)
	for key, pkgSets := range imgType.PackageSets {
		var rpmmdPkgSet rpmmd.PackageSet
		for _, pkgSet := range pkgSets {
			rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
				Include: pkgSet.Include,
				Exclude: pkgSet.Exclude,
			})

			if pkgSet.Condition != nil {
				// process conditions
				if archSet, ok := pkgSet.Condition.Architecture[archName]; ok {
					rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
						Include: archSet.Include,
						Exclude: archSet.Exclude,
					})
				}
				if distroNameSet, ok := pkgSet.Condition.DistroName[distroName]; ok {
					rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
						Include: distroNameSet.Include,
						Exclude: distroNameSet.Exclude,
					})
				}
				// note that we don't need to order here, as
				// packageSets are strictly additive the order
				// is irrelevant
				for ltVer, ltSet := range pkgSet.Condition.VersionLessThan {
					if r, ok := replacements[ltVer]; ok {
						ltVer = r
					}
					if common.VersionLessThan(distroVersion, ltVer) {
						rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
							Include: ltSet.Include,
							Exclude: ltSet.Exclude,
						})
					}
				}

				for gteqVer, gteqSet := range pkgSet.Condition.VersionGreaterOrEqual {
					if r, ok := replacements[gteqVer]; ok {
						gteqVer = r
					}
					if common.VersionGreaterThanOrEqual(distroVersion, gteqVer) {
						rpmmdPkgSet = rpmmdPkgSet.Append(rpmmd.PackageSet{
							Include: gteqSet.Include,
							Exclude: gteqSet.Exclude,
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

	return res, nil
}

// PartitionTable returns the partionTable for the given distro/imgType.
func PartitionTable(it distro.ImageType, replacements map[string]string) (*disk.PartitionTable, error) {
	distroNameVer := it.Arch().Distro().Name()

	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	imgType, ok := toplevel.ImageTypes[it.Name()]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, it.Name())
	}
	if imgType.PartitionTables == nil {
		return nil, fmt.Errorf("%w: %q", ErrNoPartitionTableForImgType, it.Name())
	}
	arch := it.Arch()
	archName := arch.Name()

	if imgType.PartitionTablesOverrides != nil {
		cond := imgType.PartitionTablesOverrides.Condition
		distroName, distroVersion := splitDistroNameVer(it.Arch().Distro().Name())

		for _, ltVer := range versionLessThanSortedKeys(cond.VersionLessThan) {
			ltOverrides := cond.VersionLessThan[ltVer]
			if r, ok := replacements[ltVer]; ok {
				ltVer = r
			}
			if common.VersionLessThan(distroVersion, ltVer) {
				for arch, overridePt := range ltOverrides {
					imgType.PartitionTables[arch] = overridePt
				}
			}
		}

		for _, gteqVer := range backward(versionLessThanSortedKeys(cond.VersionGreaterOrEqual)) {
			geOverrides := cond.VersionGreaterOrEqual[gteqVer]
			if r, ok := replacements[gteqVer]; ok {
				gteqVer = r
			}
			if common.VersionGreaterThanOrEqual(distroVersion, gteqVer) {
				for arch, overridePt := range geOverrides {
					imgType.PartitionTables[arch] = overridePt
				}
			}
		}

		if distroNameOverrides, ok := cond.DistroName[distroName]; ok {
			for arch, overridePt := range distroNameOverrides {
				imgType.PartitionTables[arch] = overridePt
			}
		}
	}

	pt, ok := imgType.PartitionTables[archName]
	if !ok {
		return nil, fmt.Errorf("%w (%q): %q", ErrNoPartitionTableForArch, it.Name(), archName)
	}

	return pt, nil
}

func splitDistroNameVer(distroNameVer string) (string, string) {
	// we need to split from the right for "centos-stream-10" like
	// distro names, sadly go has no rsplit() so we do it manually
	// XXX: we cannot use distroidparser here because of import cycles
	idx := strings.LastIndex(distroNameVer, "-")
	return distroNameVer[:idx], distroNameVer[idx+1:]
}

func load(distroNameVer string) (*toplevelYAML, error) {
	// we need to split from the right for "centos-stream-10" like
	// distro names, sadly go has no rsplit() so we do it manually
	// XXX: we cannot use distroidparser here because of import cycles
	distroName, distroVersion := splitDistroNameVer(distroNameVer)
	distroNameMajorVer := strings.SplitN(distroNameVer, ".", 2)[0]
	distroMajorVer := strings.SplitN(distroVersion, ".", 2)[0]

	// XXX: this is a short term measure, pass a set of
	// searchPaths down the stack instead
	var dataFS fs.FS = DataFS
	if overrideDir := experimentalflags.String("yamldir"); overrideDir != "" {
		olog.Printf("WARNING: using experimental override dir %q", overrideDir)
		dataFS = os.DirFS(overrideDir)
	}

	// XXX: this is only needed temporary until we have a "distros.yaml"
	// that describes some high-level properties of each distro
	// (like their yaml dirs)
	var baseDir string
	switch distroName {
	case "rhel":
		// rhel yaml files are under ./rhel-$majorVer
		baseDir = distroNameMajorVer
	case "almalinux":
		// almalinux yaml is just rhel, we take only its major version
		baseDir = fmt.Sprintf("rhel-%s", distroMajorVer)
	case "centos", "almalinux_kitten":
		// centos and kitten yaml is just rhel but we have (sadly) no
		// symlinks in "go:embed" so we have to have this slightly ugly
		// workaround
		baseDir = fmt.Sprintf("rhel-%s", distroVersion)
	case "fedora", "test-distro":
		// our other distros just have a single yaml dir per distro
		// and use condition.version_gt etc
		baseDir = distroName
	default:
		return nil, fmt.Errorf("unsupported distro in loader %q (add to loader.go)", distroName)
	}

	f, err := dataFS.Open(filepath.Join(baseDir, "distro.yaml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)

	// each imagetype can have multiple package sets, so that we can
	// use yaml aliases/anchors to de-duplicate them
	var toplevel toplevelYAML
	if err := decoder.Decode(&toplevel); err != nil {
		return nil, err
	}

	return &toplevel, nil
}

// ImageConfig returns the image type specific ImageConfig
func ImageConfig(distroNameVer, archName, typeName string, replacements map[string]string) (*distro.ImageConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	imgConfig := imgType.ImageConfig.ImageConfig
	cond := imgType.ImageConfig.Condition
	if cond != nil {
		distroName, distroVersion := splitDistroNameVer(distroNameVer)

		if distroNameCnf, ok := cond.DistroName[distroName]; ok {
			imgConfig = distroNameCnf.InheritFrom(imgConfig)
		}
		if archCnf, ok := cond.Architecture[archName]; ok {
			imgConfig = archCnf.InheritFrom(imgConfig)
		}
		for ltVer, ltConf := range cond.VersionLessThan {
			if r, ok := replacements[ltVer]; ok {
				ltVer = r
			}
			if common.VersionLessThan(distroVersion, ltVer) {
				imgConfig = ltConf.InheritFrom(imgConfig)
			}
		}
	}

	return imgConfig, nil
}

// nNonEmpty returns the number of non-empty maps in the given
// input
func nNonEmpty[K comparable, V any](maps ...map[K]V) int {
	var nonEmpty int
	for _, m := range maps {
		if len(m) > 0 {
			nonEmpty++
		}
	}
	return nonEmpty
}

// InstallerConfig returns the InstallerConfig for the given imgType
// Note that on conditions the InstallerConfig is fully replaced, do
// any merging in YAML
func InstallerConfig(distroNameVer, archName, typeName string, replacements map[string]string) (*distro.InstallerConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	installerConfig := imgType.InstallerConfig.InstallerConfig
	cond := imgType.InstallerConfig.Condition
	if cond != nil {
		if nNonEmpty(cond.DistroName, cond.Architecture, cond.VersionLessThan) > 1 {
			return nil, fmt.Errorf("only a single conditional allowed in installer config for %v", typeName)
		}

		distroName, distroVersion := splitDistroNameVer(distroNameVer)

		if distroNameCnf, ok := cond.DistroName[distroName]; ok {
			installerConfig = distroNameCnf
		}
		if archCnf, ok := cond.Architecture[archName]; ok {
			installerConfig = archCnf
		}
		for ltVer, ltConf := range cond.VersionLessThan {
			if r, ok := replacements[ltVer]; ok {
				ltVer = r
			}
			if common.VersionLessThan(distroVersion, ltVer) {
				installerConfig = ltConf
			}
		}
	}

	return installerConfig, nil
}

func ImageTypes(distroNameVer string) (map[string]ImageTypeYAML, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	// We have a bunch of names like "server-ami" that are writen
	// in the YAML as "server_ami" so we need to normalize
	imgTypes := make(map[string]ImageTypeYAML, len(toplevel.ImageTypes))
	for name := range toplevel.ImageTypes {
		v := toplevel.ImageTypes[name]
		v.name = name
		imgTypes[name] = v
	}

	return imgTypes, nil
}
