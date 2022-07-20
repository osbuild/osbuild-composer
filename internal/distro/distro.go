package distro

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	// architecture names

	X86_64ArchName  = "x86_64"
	Aarch64ArchName = "aarch64"
	Ppc64leArchName = "ppc64le"
	S390xArchName   = "s390x"
)

type BootType string

const (
	UnsetBootType  BootType = ""
	LegacyBootType BootType = "legacy"
	UEFIBootType   BootType = "uefi"
	HybridBootType BootType = "hybrid"
)

// A Distro represents composer's notion of what a given distribution is.
type Distro interface {
	// Returns the name of the distro.
	Name() string

	// Returns the release version of the distro. This is used in repo
	// files on the host system and required for the subscription support.
	Releasever() string

	// Returns the module platform id of the distro. This is used by DNF
	// for modularity support.
	ModulePlatformID() string

	// Returns the ostree reference template
	OSTreeRef() string

	// Returns a sorted list of the names of the architectures this distro
	// supports.
	ListArches() []string

	// Returns an object representing the given architecture as support
	// by this distro.
	GetArch(arch string) (Arch, error)
}

// An Arch represents a given distribution's support for a given architecture.
type Arch interface {
	// Returns the name of the architecture.
	Name() string

	// Returns a sorted list of the names of the image types this architecture
	// supports.
	ListImageTypes() []string

	// Returns an object representing a given image format for this architecture,
	// on this distro.
	GetImageType(imageType string) (ImageType, error)

	// Returns the parent distro
	Distro() Distro
}

// An ImageType represents a given distribution's support for a given Image Type
// for a given architecture.
type ImageType interface {
	// Returns the name of the image type.
	Name() string

	// Returns the parent architecture
	Arch() Arch

	// Returns the canonical filename for the image type.
	Filename() string

	// Retrns the MIME-type for the image type.
	MIMEType() string

	// Returns the default OSTree ref for the image type.
	OSTreeRef() string

	// Returns the proper image size for a given output format. If the input size
	// is 0 the default value for the format will be returned.
	Size(size uint64) uint64

	// Returns the corresponding partion type ("gpt", "dos") or "" the image type
	// has no partition table. Only support for RHEL 8.5+
	PartitionType() string

	// Returns the sets of packages to include and exclude when building the image.
	// Indexed by a string label. How each set is labeled and used depends on the
	// image type.
	PackageSets(bp blueprint.Blueprint, options ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet

	// Returns the names of the pipelines that set up the build environment (buildroot).
	BuildPipelines() []string

	// Returns the names of the pipelines that create the image.
	PayloadPipelines() []string

	// Returns the package set names safe to install custom packages via custom repositories.
	PayloadPackageSets() []string

	// Returns named arrays of package set names which should be depsolved in a chain.
	PackageSetsChains() map[string][]string

	// Returns the names of the stages that will produce the build output.
	Exports() []string

	// Returns an osbuild manifest, containing the sources and pipeline necessary
	// to build an image, given output format with all packages and customizations
	// specified in the given blueprint. The packageSpecSets must be labelled in
	// the same way as the originating PackageSets.
	Manifest(b *blueprint.Customizations, options ImageOptions, repos []rpmmd.RepoConfig, packageSpecSets map[string][]rpmmd.PackageSpec, containers []container.Spec, seed int64) (Manifest, error)
}

// The ImageOptions specify options for a specific image build
type ImageOptions struct {
	OSTree       ostree.RequestParams
	Size         uint64
	Subscription *SubscriptionImageOptions
}

// The SubscriptionImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type SubscriptionImageOptions struct {
	Organization  string
	ActivationKey string
	ServerUrl     string
	BaseUrl       string
	Insights      bool
}

type BasePartitionTableMap map[string]disk.PartitionTable

// A Manifest is an opaque JSON object, which is a valid input to osbuild
type Manifest []byte

func (m Manifest) MarshalJSON() ([]byte, error) {
	return json.RawMessage(m).MarshalJSON()
}

func (m *Manifest) UnmarshalJSON(payload []byte) error {
	var raw json.RawMessage
	err := (&raw).UnmarshalJSON(payload)
	if err != nil {
		return err
	}
	*m = Manifest(raw)
	return nil
}

type manifestVersion struct {
	Version string `json:"version"`
}

func (m Manifest) Version() (string, error) {
	mver := new(manifestVersion)
	if err := json.Unmarshal(m, mver); err != nil {
		return "", err
	}

	switch mver.Version {
	case "":
		return "1", nil
	case "2":
		return "2", nil
	default:
		return "", fmt.Errorf("Unsupported Manifest version %s", mver.Version)
	}
}

// Fallbacks: When a new method is added to an interface to provide to provide
// information that isn't available for older implementations, the older
// methods should return a fallback/default value by calling the appropriate
// function from below.
// Example: Exports() simply returns "assembler" for older image type
// implementations that didn't produce v1 manifests that have named pipelines.
func BuildPipelinesFallback() []string {
	return []string{"build"}
}

func PayloadPipelinesFallback() []string {
	return []string{"os", "assembler"}
}

func ExportsFallback() []string {
	return []string{"assembler"}
}

func PayloadPackageSets() []string {
	return []string{}
}

func MakePackageSetChains(t ImageType, packageSets map[string]rpmmd.PackageSet, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	allSetNames := make([]string, len(packageSets))
	idx := 0
	for setName := range packageSets {
		allSetNames[idx] = setName
		idx++
	}

	// map repository PackageSets to the list of repo configs
	packageSetsRepos := make(map[string][]rpmmd.RepoConfig)
	for idx := range repos {
		repo := repos[idx]
		if len(repo.PackageSets) == 0 {
			// repos that don't specify package sets get used everywhere
			repo.PackageSets = allSetNames
		}
		for _, name := range repo.PackageSets {
			psRepos := packageSetsRepos[name]
			psRepos = append(psRepos, repo)
			packageSetsRepos[name] = psRepos
		}
	}

	chainedSets := make(map[string][]rpmmd.PackageSet)
	addedSets := make(map[string]bool)
	// first collect package sets that are part of a chain
	for specName, setNames := range t.PackageSetsChains() {
		pkgSets := make([]rpmmd.PackageSet, len(setNames))

		// add package-set-specific repositories to each set if one is defined
		for idx, pkgSetName := range setNames {
			pkgSet, ok := packageSets[pkgSetName]
			if !ok {
				panic(fmt.Sprintf("image type %q specifies chained package set %q but no package set with that name exists", t.Name(), pkgSetName))
			}
			pkgSet.Repositories = packageSetsRepos[pkgSetName]
			pkgSets[idx] = pkgSet
			addedSets[pkgSetName] = true
		}
		chainedSets[specName] = pkgSets
	}

	// add the rest of the package sets
	for name, pkgSet := range packageSets {
		if addedSets[name] {
			// already added
			continue
		}
		pkgSet.Repositories = packageSetsRepos[name]
		chainedSets[name] = []rpmmd.PackageSet{pkgSet}
		addedSets[name] = true // NOTE: not really necessary but good book-keeping in case this function gets expanded
	}

	return chainedSets
}

func IsMountpointAllowed(mountpoint string, allowlist []string) bool {
	for _, allowed := range allowlist {
		match, _ := path.Match(allowed, mountpoint)
		if match {
			return true
		}
		// ensure that only clean mountpoints
		// are valid
		if strings.Contains(mountpoint, "//") {
			return false
		}
		match = strings.HasPrefix(mountpoint, allowed+"/")
		if allowed != "/" && match {
			return true
		}
	}
	return false
}
