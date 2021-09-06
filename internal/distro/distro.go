package distro

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
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

	// Returns the sets of packages to include and exclude when building the image.
	// Indexed by a string label. How each set is labeled and used depends on the
	// image type.
	PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet

	// Returns the names of the pipelines that set up the build environment (buildroot).
	BuildPipelines() []string

	// Returns the names of the pipelines that create the image.
	PayloadPipelines() []string

	// Returns the names of the stages that will produce the build output.
	Exports() []string

	// Returns an osbuild manifest, containing the sources and pipeline necessary
	// to build an image, given output format with all packages and customizations
	// specified in the given blueprint. The packageSpecSets must be labelled in
	// the same way as the originating PackageSets.
	Manifest(b *blueprint.Customizations, options ImageOptions, repos []rpmmd.RepoConfig, packageSpecSets map[string][]rpmmd.PackageSpec, seed int64) (Manifest, error)
}

// The ImageOptions specify options for a specific image build
type ImageOptions struct {
	OSTree       OSTreeImageOptions
	Size         uint64
	Subscription *SubscriptionImageOptions
}

// The OSTreeImageOptions specify ostree-specific image options
type OSTreeImageOptions struct {
	Ref    string
	Parent string
	URL    string
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

func GetHostDistroName() (string, bool, bool, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", false, false, err
	}
	defer f.Close()
	osrelease, err := readOSRelease(f)
	if err != nil {
		return "", false, false, err
	}

	isStream := osrelease["NAME"] == "CentOS Stream"

	// NOTE: We only consider major releases up until rhel 8.4
	version := strings.Split(osrelease["VERSION_ID"], ".")
	name := osrelease["ID"] + "-" + version[0]
	if osrelease["ID"] == "rhel" && ((version[0] == "8" && version[1] >= "4") || version[0] == "9") {
		name = name + version[1]
	}

	// TODO: We should probably index these things by the full CPE
	beta := strings.Contains(osrelease["CPE_NAME"], "beta")
	return name, beta, isStream, nil
}

// GetRedHatRelease returns the content of /etc/redhat-release
// without the trailing new-line.
func GetRedHatRelease() (string, error) {
	raw, err := ioutil.ReadFile("/etc/redhat-release")
	if err != nil {
		return "", fmt.Errorf("cannot read /etc/redhat-release: %v", err)
	}

	//Remove the trailing new-line.
	redHatRelease := strings.TrimSpace(string(raw))

	return redHatRelease, nil
}

func readOSRelease(r io.Reader) (map[string]string, error) {
	osrelease := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("readOSRelease: invalid input")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value[0] == '"' {
			if len(value) < 2 || value[len(value)-1] != '"' {
				return nil, errors.New("readOSRelease: invalid input")
			}
			value = value[1 : len(value)-1]
		}

		osrelease[key] = value
	}

	return osrelease, nil
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
