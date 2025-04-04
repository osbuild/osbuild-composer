// package defs contain the distro definitions used by the "images" library
package defs

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/experimentalflags"
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
	ImageConfig imageConfig          `yaml:"image_config,omitempty"`
	ImageTypes  map[string]imageType `yaml:"image_types"`
	Common      map[string]any       `yaml:".common,omitempty"`
}

type imageConfig struct {
	Default   *distro.ImageConfig    `yaml:"default"`
	Condition *imageConfigConditions `yaml:"condition,omitempty"`
}

type imageConfigConditions struct {
	DistroName map[string]*distro.ImageConfig `yaml:"distro_name,omitempty"`
}

type imageType struct {
	PackageSets []packageSet `yaml:"package_sets"`
	// archStr->partitionTable
	PartitionTables map[string]*disk.PartitionTable `yaml:"partition_table"`
	// override specific aspects of the partition table
	PartitionTablesOverrides *partitionTablesOverrides `yaml:"partition_table_override"`
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
	Conditional *partitionTablesOverwriteConditional `yaml:"condition"`
}

func (po *partitionTablesOverrides) Apply(it distro.ImageType, pt *disk.PartitionTable, replacements map[string]string) error {
	if po == nil {
		return nil
	}
	cond := po.Conditional
	_, distroVersion := splitDistroNameVer(it.Arch().Distro().Name())

	for gteqVer, geOverrides := range cond.VersionGreaterOrEqual {
		if r, ok := replacements[gteqVer]; ok {
			gteqVer = r
		}
		if common.VersionGreaterThanOrEqual(distroVersion, gteqVer) {
			for _, overrideOp := range geOverrides {
				if err := overrideOp.Apply(pt); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type partitionTablesOverwriteConditional struct {
	VersionGreaterOrEqual map[string][]partitionTablesOverrideOp `yaml:"version_greater_or_equal,omitempty"`
}

type partitionTablesOverrideOp struct {
	PartitionIndex int    `yaml:"partition_index"`
	Size           uint64 `yaml:"size"`
	FSTabOptions   string `yaml:"fstab_options"`
}

func (op *partitionTablesOverrideOp) Apply(pt *disk.PartitionTable) error {
	selectPart := op.PartitionIndex
	if selectPart > len(pt.Partitions) {
		return fmt.Errorf("override %q part %v outside of partitionTable %+v", op, selectPart, pt)
	}
	if op.Size > 0 {
		pt.Partitions[selectPart].Size = op.Size
	}
	if op.FSTabOptions != "" {
		part := pt.Partitions[selectPart]
		fs, ok := part.Payload.(*disk.Filesystem)
		if !ok {
			return fmt.Errorf("override %q part %v for fstab_options expecting filesystem got %T", op, selectPart, part)
		}
		fs.FSTabOptions = op.FSTabOptions
	}

	return nil
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

// PackageSet loads the PackageSet from the yaml source file discovered via the
// imagetype. By default the imagetype name is used to load the packageset
// but with "overrideTypeName" this can be overriden (useful for e.g.
// installer image types).
func PackageSet(it distro.ImageType, overrideTypeName string, replacements map[string]string) (rpmmd.PackageSet, error) {
	typeName := it.Name()
	if overrideTypeName != "" {
		typeName = overrideTypeName
	}
	typeName = strings.ReplaceAll(typeName, "-", "_")

	arch := it.Arch()
	archName := arch.Name()
	distribution := arch.Distro()
	distroNameVer := distribution.Name()
	distroName, distroVersion := splitDistroNameVer(distroNameVer)

	// each imagetype can have multiple package sets, so that we can
	// use yaml aliases/anchors to de-duplicate them
	toplevel, err := load(distroNameVer)
	if err != nil {
		return rpmmd.PackageSet{}, err
	}

	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return rpmmd.PackageSet{}, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}

	var rpmmdPkgSet rpmmd.PackageSet
	for _, pkgSet := range imgType.PackageSets {
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

	return rpmmdPkgSet, nil
}

// PartitionTable returns the partionTable for the given distro/imgType.
func PartitionTable(it distro.ImageType, replacements map[string]string) (*disk.PartitionTable, error) {
	distroNameVer := it.Arch().Distro().Name()
	typeName := strings.ReplaceAll(it.Name(), "-", "_")

	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	if imgType.PartitionTables == nil {
		return nil, fmt.Errorf("%w: %q", ErrNoPartitionTableForImgType, typeName)
	}
	arch := it.Arch()
	archName := arch.Name()

	pt, ok := imgType.PartitionTables[archName]
	if !ok {
		return nil, fmt.Errorf("%w (%q): %q", ErrNoPartitionTableForArch, typeName, archName)
	}

	if err := imgType.PartitionTablesOverrides.Apply(it, pt, replacements); err != nil {
		return nil, err
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

	// XXX: this is a short term measure, pass a set of
	// searchPaths down the stack instead
	var dataFS fs.FS = DataFS
	if overrideDir := experimentalflags.String("yamldir"); overrideDir != "" {
		logrus.Warnf("using experimental override dir %q", overrideDir)
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
	case "centos":
		// centos yaml is just rhel but we have (sadly) no symlinks
		// in "go:embed" so we have to have this slightly ugly
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
