// package defs contain the distro definitions used by the "images" library
package defs

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"text/template"

	"github.com/gobwas/glob"
	"gopkg.in/yaml.v3"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var (
	ErrImageTypeNotFound          = errors.New("image type not found")
	ErrNoPartitionTableForImgType = errors.New("no partition table for image type")
	ErrNoPartitionTableForArch    = errors.New("no partition table for arch")
)

//go:embed *.yaml */*.yaml
var data embed.FS

var defaultDataFS fs.FS = data

// distrosYAML defines all supported YAML based distributions
type distrosYAML struct {
	Distros []DistroYAML
}

func dataFS() fs.FS {
	// XXX: this is a short term measure, pass a set of
	// searchPaths down the stack instead
	var dataFS fs.FS = defaultDataFS
	if overrideDir := experimentalflags.String("yamldir"); overrideDir != "" {
		olog.Printf("WARNING: using experimental override dir %q", overrideDir)
		dataFS = os.DirFS(overrideDir)
	}
	return dataFS
}

type DistroYAML struct {
	// Match can be used to match multiple versions via a
	// fnmatch/glob style expression. We could also use a
	// regex and do something like:
	//   rhel-(?P<major>[0-9]+)\.(?P<minor>[0-9]+)
	// if we need to be more precise in the future, but for
	// now every match will be split into "$distroname-$major.$minor"
	// (with minor being optional)
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
}

func executeTemplates(d *DistroYAML, nameVer string) error {
	id, err := distro.ParseID(nameVer)
	if err != nil {
		return err
	}

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

// Distro return the given distro or nil if the distro is not
// found. This mimics the "distrofactory.GetDistro() interface.
//
// Note that eventually we want something like "Distros()" instead
// that returns all known distros but for now we keep compatibility
// with the way distrofactory/reporegistry work which is by defining
// distros via repository files.
func Distro(nameVer string) (*DistroYAML, error) {
	f, err := dataFS().Open("distros.yaml")
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

	for _, distro := range distros.Distros {
		if distro.Name == nameVer {
			return &distro, nil
		}

		pat, err := glob.Compile(distro.Match)
		if err != nil {
			return nil, err
		}
		if pat.Match(nameVer) {
			if err := executeTemplates(&distro, nameVer); err != nil {
				return nil, err
			}

			return &distro, nil
		}
	}

	return nil, nil
}

// imageTypesYAML describes the image types for a given distribution
// family. Note that multiple distros may use the same image types,
// e.g. centos/rhel
type imageTypesYAML struct {
	ImageConfig distroImageConfig    `yaml:"image_config,omitempty"`
	ImageTypes  map[string]imageType `yaml:"image_types"`
	Common      map[string]any       `yaml:".common,omitempty"`
}

type distroImageConfig struct {
	Default    *distro.ImageConfig                     `yaml:"default"`
	Conditions map[string]*distroImageConfigConditions `yaml:"conditions,omitempty"`
}

// multiple whenConditions are considred AND
type whenCondition struct {
	DistroName            string `yaml:"distro_name,omitempty"`
	Architecture          string `yaml:"arch,omitempty"`
	VersionLessThan       string `yaml:"version_less_than,omitempty"`
	VersionGreaterOrEqual string `yaml:"version_greater_or_equal,omitempty"`
	VersionEqual          string `yaml:"version_equal,omitempty"`
}

func (wc *whenCondition) Eval(id *distro.ID, archStr string) bool {
	match := true

	if wc.DistroName != "" {
		match = match && (wc.DistroName == id.Name)
	}
	if wc.Architecture != "" {
		match = match && (wc.Architecture == archStr)
	}
	if wc.VersionLessThan != "" {
		match = match && (common.VersionLessThan(versionStringForVerCmp(*id), wc.VersionLessThan))
	}
	if wc.VersionGreaterOrEqual != "" {
		match = match && (common.VersionGreaterThanOrEqual(versionStringForVerCmp(*id), wc.VersionGreaterOrEqual))
	}
	if wc.VersionEqual != "" {
		match = match && (id.VersionString() == wc.VersionEqual)
	}

	return match
}

type distroImageConfigConditions struct {
	When  whenCondition       `yaml:"when,omitempty"`
	Merge *distro.ImageConfig `yaml:"merge,omitempty"`
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

	BootISO  bool   `yaml:"boot_iso"`
	ISOLabel string `yaml:"iso_label"`
	// XXX: or iso_variant?
	Variant string `yaml:"variant"`

	RPMOSTree bool `yaml:"rpm_ostree"`

	OSTree struct {
		Name   string `yaml:"name"`
		Remote string `yaml:"remote"`
	} `yaml:"ostree"`

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
	Conditions          map[string]*conditionsImgConf `yaml:"conditions,omitempty"`
}

type conditionsImgConf struct {
	When  whenCondition       `yaml:"when,omitempty"`
	Merge *distro.ImageConfig `yaml:"merge"`
}

type installerConfig struct {
	*distro.InstallerConfig `yaml:",inline"`
	Conditions              map[string]*conditionsInstallerConf `yaml:"conditions,omitempty"`
}

type conditionsInstallerConf struct {
	When     whenCondition           `yaml:"when,omitempty"`
	Override *distro.InstallerConfig `yaml:"override,omitempty"`
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
// TODO: this should become an explicit chose in "distro.yaml" but until
// we have everything converted to generic.Distro accessing the properites
// from an image type is very hard so we start here.
func versionStringForVerCmp(u distro.ID) string {
	if u.MinorVersion == -1 {
		u.MinorVersion = 999
	}
	return u.VersionString()
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

	condMap := toplevel.ImageConfig.Conditions
	if condMap != nil {
		id, err := distro.ParseID(distroNameVer)
		if err != nil {
			return nil, err
		}
		for _, cond := range condMap {
			if cond.When.Eval(id, "") {
				imgConfig = cond.Merge.InheritFrom(imgConfig)
			}
		}
	}

	return imgConfig, nil
}

// PackageSets loads the PackageSets from the yaml source file
// discovered via the imagetype.
func PackageSets(it distro.ImageType) (map[string]rpmmd.PackageSet, error) {
	typeName := it.Name()

	arch := it.Arch()
	archName := arch.Name()
	distribution := arch.Distro()
	distroNameVer := distribution.Name()

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

			if pkgSet.Conditions != nil {
				id, err := distro.ParseID(distroNameVer)
				if err != nil {
					return nil, err
				}

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

	return res, nil
}

// PartitionTable returns the partionTable for the given distro/imgType.
func PartitionTable(it distro.ImageType) (*disk.PartitionTable, error) {
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

	pt, ok := imgType.PartitionTables[archName]
	if !ok {
		return nil, fmt.Errorf("%w (%q): %q", ErrNoPartitionTableForArch, it.Name(), archName)
	}

	if imgType.PartitionTablesOverrides != nil {
		id, err := distro.ParseID(it.Arch().Distro().Name())
		if err != nil {
			return nil, err
		}
		for _, cond := range imgType.PartitionTablesOverrides.Conditions {
			if cond.When.Eval(id, archName) {
				pt = cond.Override[archName]
			}
		}
	}

	return pt, nil
}

// Cache the toplevel structure, loading/parsing YAML is quite
// expensive. This can all be removed in the future where there
// is a single load for each distroNameVer. Right now the various
// helpers (like ParititonTable(), ImageConfig() are called a
// gazillion times. However once we move into the "generic" distro
// the distro will do a single load/parse of all image types and
// just reuse them and this can go.
type imageTypesCache struct {
	cache map[string]*imageTypesYAML
	mu    sync.Mutex
}

func newImageTypesCache() *imageTypesCache {
	return &imageTypesCache{cache: make(map[string]*imageTypesYAML)}
}

func (i *imageTypesCache) Get(hash string) *imageTypesYAML {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.cache[hash]
}

func (i *imageTypesCache) Set(hash string, ity *imageTypesYAML) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cache[hash] = ity
}

var (
	itCache = newImageTypesCache()
)

func load(distroNameVer string) (*imageTypesYAML, error) {
	id, err := distro.ParseID(distroNameVer)
	if err != nil {
		return nil, err
	}

	// XXX: this is only needed temporary until we have a "distros.yaml"
	// that describes some high-level properties of each distro
	// (like their yaml dirs)
	var baseDir string
	switch id.Name {
	case "rhel", "almalinux", "centos", "almalinux_kitten":
		// rhel yaml files are under ./rhel-$majorVer
		// almalinux yaml is just rhel, we take only its major version
		// centos and kitten yaml is just rhel but we have (sadly) no
		// symlinks in "go:embed" so we have to have this slightly ugly
		// workaround
		baseDir = fmt.Sprintf("rhel-%v", id.MajorVersion)
	case "test-distro":
		// our other distros just have a single yaml dir per distro
		// and use condition.version_gt etc
		baseDir = id.Name
	}

	// take the base path from the distros.yaml
	distro, err := Distro(distroNameVer)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if distro != nil && distro.DefsPath != "" {
		baseDir = distro.DefsPath
	}

	f, err := dataFS().Open(filepath.Join(baseDir, "distro.yaml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// XXX: this is currently needed because rhel distros call
	// ImageType() and ParitionTable() a gazillion times and
	// each time the full yaml is loaded. Once things move to
	// the "generic" distro this will no longer be the case and
	// this cache can be removed and below we can decode directly
	// from "f" again instead of wasting memory with "buf"
	var buf bytes.Buffer
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(&buf, h), f); err != nil {
		return nil, fmt.Errorf("cannot read from %s: %w", baseDir, err)
	}
	inputHash := string(h.Sum(nil))
	if cached := itCache.Get(inputHash); cached != nil {
		return cached, nil
	}

	var toplevel imageTypesYAML
	decoder := yaml.NewDecoder(&buf)
	decoder.KnownFields(true)
	if err := decoder.Decode(&toplevel); err != nil {
		return nil, err
	}

	// XXX: remove once we no longer need caching
	itCache.Set(inputHash, &toplevel)

	return &toplevel, nil
}

// ImageConfig returns the image type specific ImageConfig
func ImageConfig(distroNameVer, archName, typeName string) (*distro.ImageConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	imgConfig := imgType.ImageConfig.ImageConfig
	if imgType.ImageConfig.Conditions != nil {
		id, err := distro.ParseID(distroNameVer)
		if err != nil {
			return nil, err
		}

		for _, cond := range imgType.ImageConfig.Conditions {
			if cond.When.Eval(id, archName) {
				imgConfig = cond.Merge.InheritFrom(imgConfig)
			}
		}
	}

	return imgConfig, nil
}

// InstallerConfig returns the InstallerConfig for the given imgType
// Note that on conditions the InstallerConfig is fully replaced, do
// any merging in YAML
func InstallerConfig(distroNameVer, archName, typeName string) (*distro.InstallerConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	installerConfig := imgType.InstallerConfig.InstallerConfig
	if imgType.InstallerConfig.Conditions != nil {
		for _, cond := range imgType.InstallerConfig.Conditions {
			id, err := distro.ParseID(distroNameVer)
			if err != nil {
				return nil, err
			}
			if cond.When.Eval(id, archName) {
				installerConfig = cond.Override
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
