package rhel7

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/image"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/pathpolicy"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/workload"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageFunc func(workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.Spec, rng *rand.Rand) (image.ImageKind, error)

type imageType struct {
	arch               *architecture
	platform           platform.Platform
	environment        environment.Environment
	workload           workload.Workload
	name               string
	nameAliases        []string
	filename           string
	compression        string // TODO: remove from image definition and make it a transport option
	mimeType           string
	packageSets        map[string]packageSetFunc
	packageSetChains   map[string][]string
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	image              imageFunc

	// bootable image
	bootable bool
	// List of valid arches for the image type
	basePartitionTables distro.BasePartitionTableMap
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
	// Not supported
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *imageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) PackageSetsChains() map[string][]string {
	return t.packageSetChains
}

func (t *imageType) Exports() []string {
	if len(t.exports) == 0 {
		panic(fmt.Sprintf("programming error: no exports for '%s'", t.name))
	}
	return t.exports
}

func (t *imageType) BootMode() distro.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return distro.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return distro.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return distro.BOOT_LEGACY
	}
	return distro.BOOT_NONE
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.basePartitionTables[archName]

	if !exists {
		return nil, fmt.Errorf("unknown arch: " + archName)
	}

	imageSize := t.Size(options.Size)

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, true, nil, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.getDefaultImageConfig())

}

func (t *imageType) PartitionType() string {
	archName := t.arch.Name()
	basePartitionTable, exists := t.basePartitionTables[archName]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecs map[string][]rpmmd.PackageSpec,
	containers []container.Spec,
	seed int64) (manifest.OSBuildManifest, []string, error) {

	bp := &blueprint.Blueprint{Name: "empty blueprint"}
	err := bp.Initialize()
	if err != nil {
		panic("could not initialize empty blueprint: " + err.Error())
	}
	bp.Customizations = customizations

	// the os pipeline filters repos based on the `osPkgsKey` package set, merge the repos which
	// contain a payload package set into the `osPkgsKey`, so those repos are included when
	// building the rpm stage in the os pipeline
	// TODO: roll this into workloads
	mergedRepos := make([]rpmmd.RepoConfig, 0, len(repos))
	for _, repo := range repos {
		for _, pkgsKey := range t.PayloadPackageSets() {
			// If the repo already contains the osPkgsKey, skip
			if slices.Contains(repo.PackageSets, osPkgsKey) {
				break
			}
			if slices.Contains(repo.PackageSets, pkgsKey) {
				repo.PackageSets = append(repo.PackageSets, osPkgsKey)
			}
		}
		mergedRepos = append(mergedRepos, repo)
	}
	repos = mergedRepos

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	var packageSets map[string]rpmmd.PackageSet
	w := t.workload
	if w == nil {
		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: packageSets[blueprintPkgsKey].Repositories,
			},
			Packages: bp.GetPackagesEx(false),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
		}
		w = cw
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	if t.image == nil {
		return nil, nil, nil
	}
	img, err := t.image(w, t, bp.Customizations, options, packageSets, containers, rng)
	if err != nil {
		return nil, nil, err
	}
	manifest := manifest.New()
	_, err = img.InstantiateManifest(&manifest, repos, t.arch.distro.runner, rng)
	if err != nil {
		return nil, nil, err
	}

	ret, err := manifest.Serialize(packageSpecs)
	if err != nil {
		return ret, nil, err
	}
	return ret, warnings, err
}

func (t *imageType) PackageSets(bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	packageSets := make(map[string]rpmmd.PackageSet)

	for name, getter := range t.packageSets {
		packageSets[name] = getter(t)
	}

	// amend with repository information
	for _, repo := range repos {
		if len(repo.PackageSets) > 0 {
			// only apply the repo to the listed package sets
			for _, psName := range repo.PackageSets {
				ps := packageSets[psName]
				ps.Repositories = append(ps.Repositories, repo)
				packageSets[psName] = ps
			}
		}
	}

	// Similar to above, for edge-commit and edge-container, we need to set an
	// ImageRef in order to properly initialize the manifest and package
	// selection.
	options.OSTree.ImageRef = t.OSTreeRef()

	// create a temporary container spec array with the info from the blueprint
	// to initialize the manifest
	containers := make([]container.Spec, len(bp.Containers))
	for idx := range bp.Containers {
		containers[idx] = container.Spec{
			Source:    bp.Containers[idx].Source,
			TLSVerify: bp.Containers[idx].TLSVerify,
			LocalName: bp.Containers[idx].Name,
		}
	}

	_, err := t.checkOptions(&bp, options)
	if err != nil {
		logrus.Errorf("Initializing the manifest failed for %s (%s/%s): %v", t.Name(), t.arch.distro.Name(), t.arch.Name(), err)
		return nil
	}

	w := t.workload
	if w == nil {
		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: packageSets[blueprintPkgsKey].Repositories,
			},
			Packages: bp.GetPackagesEx(false),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
		}
		w = cw
	}

	source := rand.NewSource(0)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	if t.image == nil {
		logrus.Errorf("Initializing the manifest failed for %s (%s/%s): %v", t.Name(), t.arch.distro.Name(), t.arch.Name(), err)
		return nil
	}
	img, err := t.image(w, t, bp.Customizations, options, packageSets, containers, rng)
	if err != nil {
		logrus.Errorf("Initializing the manifest failed for %s (%s/%s): %v", t.Name(), t.arch.distro.Name(), t.arch.Name(), err)
		return nil
	}
	manifest := manifest.New()
	_, err = img.InstantiateManifest(&manifest, repos, t.arch.distro.runner, rng)
	if err != nil {
		logrus.Errorf("Initializing the manifest failed for %s (%s/%s): %v", t.Name(), t.arch.distro.Name(), t.arch.Name(), err)
		return nil
	}
	return overridePackageNamesInSets(manifest.GetPackageSetChains())
}

// Runs overridePackageNames() on each package set's Include and Exclude list
// and replaces package names.
func overridePackageNamesInSets(chains map[string][]rpmmd.PackageSet) map[string][]rpmmd.PackageSet {
	pkgSetChains := make(map[string][]rpmmd.PackageSet)
	for name, chain := range chains {
		cc := make([]rpmmd.PackageSet, len(chain))
		for idx := range chain {
			cc[idx] = rpmmd.PackageSet{
				Include:      overridePackageNames(chain[idx].Include),
				Exclude:      overridePackageNames(chain[idx].Exclude),
				Repositories: chain[idx].Repositories,
			}
		}
		pkgSetChains[name] = cc
	}
	return pkgSetChains
}

// Resolve packages to their distro-specific name. This function is a temporary
// workaround to the issue of having packages specified outside of distros (in
// internal/manifest/os.go), which should be distro agnostic. In the future,
// this should be handled more generally.
func overridePackageNames(packages []string) []string {
	for idx := range packages {
		switch packages[idx] {
		case "python3-pyyaml":
			packages[idx] = "python3-PyYAML"
		}
	}
	return packages
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *imageType) checkOptions(bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	customizations := bp.Customizations
	// holds warnings (e.g. deprecation notices)
	var warnings []string
	if t.workload != nil {
		// For now, if an image type defines its own workload, don't allow any
		// user customizations.
		// Soon we will have more workflows and each will define its allowed
		// set of customizations.  The current set of customizations defined in
		// the blueprint spec corresponds to the Custom workflow.
		if customizations != nil {
			return warnings, fmt.Errorf("image type %q does not support customizations", t.name)
		}
	}

	if len(bp.Containers) > 0 {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	mountpoints := customizations.GetFilesystems()

	err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.MountpointPolicies)
	if err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		return warnings, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported os version: %s", t.arch.distro.osVersion))
	}

	// Check Directory/File Customizations are valid
	dc := customizations.GetDirectories()
	fc := customizations.GetFiles()

	err = blueprint.ValidateDirFileCustomizations(dc, fc)
	if err != nil {
		return warnings, err
	}

	err = blueprint.CheckDirectoryCustomizationsPolicy(dc, pathpolicy.CustomDirectoriesPolicies)
	if err != nil {
		return warnings, err
	}

	err = blueprint.CheckFileCustomizationsPolicy(fc, pathpolicy.CustomFilesPolicies)
	if err != nil {
		return warnings, err
	}

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return warnings, err
	}

	return warnings, nil
}
