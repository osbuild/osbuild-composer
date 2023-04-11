package fedora

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/image"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/oscap"
	"github.com/osbuild/osbuild-composer/internal/pathpolicy"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/workload"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type imageFunc func(workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.Spec, rng *rand.Rand) (image.ImageKind, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
	arch               *architecture
	platform           platform.Platform
	environment        environment.Environment
	name               string
	nameAliases        []string
	filename           string
	mimeType           string
	packageSets        map[string]packageSetFunc
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	image              imageFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: iot/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// List of valid arches for the image type
	basePartitionTables    distro.BasePartitionTableMap
	requiredPartitionSizes map[string]uint64
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
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.arch.Name())
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%common.MebiByte != 0 {
		size = (size/common.MebiByte + 1) * common.MebiByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
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

	// In case of Cloud API, this method is called before the ostree commit
	// is resolved. Unfortunately, initializeManifest when called for
	// an ostree installer returns an error.
	//
	// Work around this by providing a dummy FetchChecksum to convince the
	// method that it's fine to initialize the manifest. Note that the ostree
	// content has no effect on the package sets, so this is fine.
	//
	// See: https://github.com/osbuild/osbuild-composer/issues/3125
	//
	// TODO: Remove me when it's possible the get the package set chain without
	//       resolving the ostree reference before. Also remove the test for
	//       this workaround
	if t.rpmOstree && t.bootISO && options.OSTree.FetchChecksum == "" {
		options.OSTree.FetchChecksum = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		logrus.Warn("FIXME: Requesting package sets for iot-installer without a resolved ostree ref. Faking one.")
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

	// TODO: let image types specify valid workloads, rather than
	// always assume Custom.
	w := &workload.Custom{
		BaseWorkload: workload.BaseWorkload{
			Repos: packageSets[blueprintPkgsKey].Repositories,
		},
		Packages: bp.GetPackagesEx(false),
	}
	if services := bp.Customizations.GetServices(); services != nil {
		w.Services = services.Enabled
		w.DisabledServices = services.Disabled
	}

	source := rand.NewSource(0)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

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
	return manifest.GetPackageSetChains()
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
	return make(map[string][]string)
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
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
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return nil, fmt.Errorf("unknown arch: " + t.arch.Name())
	}

	imageSize := t.Size(options.Size)

	lvmify := !t.rpmOstree

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, lvmify, t.requiredPartitionSizes, rng)
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
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
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
	seed int64) (distro.Manifest, []string, error) {

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

	var packageSets map[string]rpmmd.PackageSet
	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// TODO: let image types specify valid workloads, rather than
	// always assume Custom.
	w := &workload.Custom{
		BaseWorkload: workload.BaseWorkload{
			Repos: packageSets[blueprintPkgsKey].Repositories,
		},
		Packages: bp.GetPackagesEx(false),
	}
	if services := bp.Customizations.GetServices(); services != nil {
		w.Services = services.Enabled
		w.DisabledServices = services.Disabled
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

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

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *imageType) checkOptions(bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {

	customizations := bp.Customizations

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.rpmOstree && (t.name != "iot-commit" && t.name != "iot-container") {
		return nil, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if t.bootISO && t.rpmOstree {
		// check the checksum instead of the URL, because the URL should have been used to resolve the checksum and we need both
		if options.OSTree.FetchChecksum == "" {
			return nil, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
	}

	if t.name == "iot-raw-image" {
		allowed := []string{"User", "Group", "Directories", "Files", "Services"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return nil, fmt.Errorf("unsupported blueprint customizations found for image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer" in RHEL distros
	}

	// BootISO's have limited support for customizations.
	// TODO: Support kernel name selection for image-installer
	if t.bootISO {
		if t.name == "iot-installer" || t.name == "image-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return nil, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return nil, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return nil, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.MountpointPolicies)
	if err != nil {
		return nil, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		supported := oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList)
		if !supported {
			return nil, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return nil, fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
		}
		if osc.DataStream == "" {
			return nil, fmt.Errorf("OpenSCAP datastream cannot be empty")
		}
		if osc.ProfileID == "" {
			return nil, fmt.Errorf("OpenSCAP profile cannot be empty")
		}
	}

	// Check Directory/File Customizations are valid
	dc := customizations.GetDirectories()
	fc := customizations.GetFiles()

	err = blueprint.ValidateDirFileCustomizations(dc, fc)
	if err != nil {
		return nil, err
	}

	err = blueprint.CheckDirectoryCustomizationsPolicy(dc, pathpolicy.CustomDirectoriesPolicies)
	if err != nil {
		return nil, err
	}

	err = blueprint.CheckFileCustomizationsPolicy(fc, pathpolicy.CustomFilesPolicies)
	if err != nil {
		return nil, err
	}

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return nil, err
	}

	return nil, nil
}
