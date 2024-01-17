package rhel9

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/pathpolicy"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// main/common os image package set name
	osPkgsKey = "os"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	// location for saving openscap remediation data
	oscapDataDir = "/oscap_data"
)

type imageFunc func(workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type basePartitionTableFunc func(t *imageType) (disk.PartitionTable, bool)

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
	// List of valid arches for the image type
	basePartitionTables basePartitionTableFunc
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

	basePartitionTable, exists := t.basePartitionTables(t)

	if !exists {
		return nil, fmt.Errorf("no partition table defined for architecture %q for image type %q", archName, t.Name())
	}

	imageSize := t.Size(options.Size)

	partitioningMode := options.PartitioningMode
	if t.rpmOstree {
		// Edge supports only LVM, force it.
		// TODO Need a central location for logic like this
		partitioningMode = disk.LVMPartitioningMode
	}

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, partitioningMode, nil, rng)
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
	basePartitionTable, exists := t.basePartitionTables(t)
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seed int64) (*manifest.Manifest, []string, error) {

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	staticPackageSets := make(map[string]rpmmd.PackageSet)

	for name, getter := range t.packageSets {
		staticPackageSets[name] = getter(t)
	}

	// amend with repository information and collect payload repos
	payloadRepos := make([]rpmmd.RepoConfig, 0)
	for _, repo := range repos {
		if len(repo.PackageSets) > 0 {
			// only apply the repo to the listed package sets
			for _, psName := range repo.PackageSets {
				if slices.Contains(t.PayloadPackageSets(), psName) {
					payloadRepos = append(payloadRepos, repo)
				}
				ps := staticPackageSets[psName]
				ps.Repositories = append(ps.Repositories, repo)
				staticPackageSets[psName] = ps
			}
		}
	}

	w := t.workload
	if w == nil {
		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: payloadRepos,
			},
			Packages: bp.GetPackagesEx(false),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
		}
		w = cw
	}

	containerSources := make([]container.SourceSpec, len(bp.Containers))
	for idx := range bp.Containers {
		containerSources[idx] = container.SourceSpec(bp.Containers[idx])
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	img, err := t.image(w, t, bp.Customizations, options, staticPackageSets, containerSources, rng)
	if err != nil {
		return nil, nil, err
	}
	mf := manifest.New()
	mf.Distro = manifest.DISTRO_EL9
	_, err = img.InstantiateManifest(&mf, repos, t.arch.distro.runner, rng)
	if err != nil {
		return nil, nil, err
	}

	return &mf, warnings, err
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

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.rpmOstree && (t.name != "edge-commit" && t.name != "edge-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if len(bp.Containers) > 0 {
		for _, container := range bp.Containers {
			if err := container.Validate(); err != nil {
				return nil, err
			}
		}
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return nil, err
		}
	}

	if t.bootISO && t.rpmOstree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return nil, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}

		if t.name == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "Ignition", "Kernel", "User", "Group", "FIPS", "Filesystem"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return warnings, fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.name)
			}

			// FDO is optional, but when specified has some restrictions
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to when using FDO", t.name)
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
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to when using FDO", t.name)
				}
			}

			// ignition is optional, we might be using FDO
			if customizations.GetIgnition() != nil {
				if customizations.GetIgnition().Embedded != nil && customizations.GetIgnition().FirstBoot != nil {
					return warnings, fmt.Errorf("both ignition embedded and firstboot configurations found")
				}
				if customizations.GetIgnition().FirstBoot != nil && customizations.GetIgnition().FirstBoot.ProvisioningURL == "" {
					return warnings, fmt.Errorf("ignition.firstboot requires a provisioning url")
				}
			}
		} else if t.name == "edge-installer" {
			allowed := []string{"User", "Group", "FIPS"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	if t.name == "edge-raw-image" || t.name == "edge-ami" || t.name == "edge-vsphere" {
		// ostree-based bootable images require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("%q images require specifying a URL from which to retrieve the OSTree commit", t.name)
		}
		allowed := []string{"Ignition", "Kernel", "User", "Group", "FIPS", "Filesystem"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return warnings, fmt.Errorf("unsupported blueprint customizations found for image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer"
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

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree && t.name != "edge-raw-image" && t.name != "edge-simplified-installer" {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	if mountpoints != nil && t.rpmOstree && (t.name == "edge-container" || t.name == "edge-commit") {
		return warnings, fmt.Errorf("Custom mountpoints are not supported for edge-container and edge-commit")
	} else if mountpoints != nil && t.rpmOstree && !(t.name == "edge-container" || t.name == "edge-commit") {
		//customization allowed for edge-raw-image,edge-ami,edge-vsphere,edge-simplified-installer
		err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.OstreeMountpointPolicies)
		if err != nil {
			return warnings, err
		}
	}

	err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.MountpointPolicies)
	if err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.arch.distro.osVersion == "9.0" {
			return warnings, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported os version: %s", t.arch.distro.osVersion))
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList) {
			return warnings, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return warnings, fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
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

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return warnings, err
	}

	return warnings, nil
}
