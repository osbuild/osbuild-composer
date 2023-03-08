package rhel8

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

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

type imageFunc func(workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.Spec, rng *rand.Rand) (image.ImageKind, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

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
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	image              imageFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: edge/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// If set to a value, it is preferred over the architecture value
	bootType distro.BootType
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
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.Arch().Name())
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
	return nil
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

// getBootType returns the BootType which should be used for this particular
// combination of architecture and image type.
func (t *imageType) getBootType() distro.BootType {
	bootType := t.arch.bootType
	if t.bootType != distro.UnsetBootType {
		bootType = t.bootType
	}
	return bootType
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.basePartitionTables[archName]

	if !exists {
		return nil, fmt.Errorf("no partition table defined for architecture %q for image type %q", archName, t.Name())
	}

	imageSize := t.Size(options.Size)

	lvmify := !t.rpmOstree

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, lvmify, nil, rng)
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

func (t *imageType) initializeManifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.Spec,
	seed int64) (*manifest.Manifest, []string, error) {

	warnings, err := t.checkOptions(bp.Customizations, options, containers)
	if err != nil {
		return nil, nil, err
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
	return &manifest, warnings, err
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string][]rpmmd.PackageSpec,
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

	manifest, warnings, err := t.initializeManifest(bp, options, mergedRepos, nil, containers, seed)
	if err != nil {
		return distro.Manifest{}, nil, err
	}

	ret, err := manifest.Serialize(packageSets)
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

	// create a manifest object and instantiate it with the computed packageSetChains
	manifest, _, err := t.initializeManifest(&bp, options, repos, packageSets, containers, 0)
	if err != nil {
		// TODO: handle manifest initialization errors more gracefully, we
		// refuse to initialize manifests with invalid config.
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
		case "python3-toml":
			packages[idx] = "python3-pytoml"
		}
	}
	return packages
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions, containers []container.Spec) ([]string, error) {
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
	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(containers) > 0 && t.rpmOstree && (t.name != "edge-commit" && t.name != "edge-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if t.bootISO && t.rpmOstree {
		// check the checksum instead of the URL, because the URL should have been used to resolve the checksum and we need both
		if options.OSTree.FetchChecksum == "" {
			return warnings, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}

		if t.name == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return warnings, fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.name)
			}
			//making fdo optional so that simplified installer can be composed w/o the FDO section in the blueprint
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to", t.name)
				}
				var diunSet int
				if customizations.GetFDO().DiunPubKeyHash != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyInsecure != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyRootCerts != "" {
					diunSet++
				}
				if diunSet != 1 {
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to", t.name)
				}
			}
		} else if t.name == "edge-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	// check the checksum instead of the URL, because the URL should have been used to resolve the checksum and we need both
	if t.name == "edge-raw-image" && options.OSTree.FetchChecksum == "" {
		return warnings, fmt.Errorf("edge raw images require specifying a URL from which to retrieve the OSTree commit")
	}

	// warn that user & group customizations on edge-commit, edge-container are deprecated
	// TODO(edge): directly error if these options are provided when rhel-9.5's time arrives
	if t.name == "edge-commit" || t.name == "edge-container" {
		if customizations.GetUsers() != nil {
			w := fmt.Sprintf("Please note that user customizations on %q image type are deprecated and will be removed in the near future\n", t.name)
			log.Print(w)
			warnings = append(warnings, w)
		}
		if customizations.GetGroups() != nil {
			w := fmt.Sprintf("Please note that group customizations on %q image type are deprecated and will be removed in the near future\n", t.name)
			log.Print(w)
			warnings = append(warnings, w)
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree && (!t.bootable || t.bootISO) {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return warnings, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.MountpointPolicies)
	if err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		// only add support for RHEL 8.7 and above. centos not supported.
		if !t.arch.distro.isRHEL() || common.VersionLessThan(t.arch.distro.osVersion, "8.7") {
			return warnings, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported os version: %s", t.arch.distro.osVersion))
		}
		supported := oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList)
		if !supported {
			return warnings, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return warnings, fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
		}
		if osc.DataStream == "" {
			return warnings, fmt.Errorf("OpenSCAP datastream cannot be empty")
		}
		if osc.ProfileID == "" {
			return warnings, fmt.Errorf("OpenSCAP profile cannot be empty")
		}
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

	return warnings, nil
}
