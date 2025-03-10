package fedora

import (
	"fmt"
	"math/rand"
	"strings"

	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/policies"
	"github.com/osbuild/images/pkg/rpmmd"
)

type imageFunc func(workload workload.Workload, t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type isoLabelFunc func(t *imageType) string

type imageType struct {
	arch                   *architecture
	platform               platform.Platform
	environment            environment.Environment
	workload               workload.Workload
	name                   string
	nameAliases            []string
	filename               string
	compression            string
	mimeType               string
	packageSets            map[string]packageSetFunc
	defaultImageConfig     *distro.ImageConfig
	defaultInstallerConfig *distro.InstallerConfig
	kernelOptions          string
	defaultSize            uint64
	buildPipelines         []string
	payloadPipelines       []string
	exports                []string
	image                  imageFunc
	isoLabel               isoLabelFunc

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

func (t *imageType) ISOLabel() (string, error) {
	if !t.bootISO {
		return "", fmt.Errorf("image type %q is not an ISO", t.name)
	}

	if t.isoLabel != nil {
		return t.isoLabel(t), nil
	}

	return "", nil
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%datasizes.MebiByte != 0 {
		size = (size/datasizes.MebiByte + 1) * datasizes.MebiByte
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

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *imageType) BootMode() platform.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return platform.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return platform.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return platform.BOOT_LEGACY
	}
	return platform.BOOT_NONE
}

func (t *imageType) getPartitionTable(
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return nil, fmt.Errorf("unknown arch for partition table: %s", t.arch.Name())
	}

	imageSize := t.Size(options.Size)
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}
	if partitioning != nil {
		// Use the new custom partition table to create a PT fully based on the user's customizations.
		// This overrides FilesystemCustomizations, but we should never have both defined.
		if options.Size > 0 {
			// user specified a size on the command line, so let's override the
			// customization with the calculated/rounded imageSize
			partitioning.MinSize = imageSize
		}

		partOptions := &disk.CustomPartitionTableOptions{
			PartitionTableType: basePartitionTable.Type, // PT type is not customizable, it is determined by the base PT for an image type or architecture
			BootMode:           t.BootMode(),
			DefaultFSType:      disk.FS_EXT4, // default fs type for Fedora
			RequiredMinSizes:   t.requiredPartitionSizes,
			Architecture:       t.platform.GetArch(),
		}
		return disk.NewCustomPartitionTable(partitioning, partOptions, rng)
	}

	partitioningMode := options.PartitioningMode
	if t.rpmOstree {
		// IoT supports only LVM, force it.
		// Raw is not supported, return an error if it is requested
		// TODO Need a central location for logic like this
		if partitioningMode == disk.RawPartitioningMode {
			return nil, fmt.Errorf("partitioning mode raw not supported for %s on %s", t.Name(), t.arch.Name())
		}
		partitioningMode = disk.AutoLVMPartitioningMode
	}

	mountpoints := customizations.GetFilesystems()
	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, partitioningMode, t.platform.GetArch(), t.requiredPartitionSizes, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.getDefaultImageConfig())

}

func (t *imageType) getDefaultInstallerConfig() (*distro.InstallerConfig, error) {
	if !t.bootISO {
		return nil, fmt.Errorf("image type %q is not an ISO", t.name)
	}

	return t.defaultInstallerConfig, nil
}

func (t *imageType) PartitionType() disk.PartitionTableType {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return disk.PT_NONE
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seedp *int64) (*manifest.Manifest, []string, error) {
	seed := distro.SeedFrom(seedp)

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	staticPackageSets := make(map[string]rpmmd.PackageSet)

	// don't add any static packages if Minimal was selected
	if !bp.Minimal {
		for name, getter := range t.packageSets {
			staticPackageSets[name] = getter(t)
		}
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
		// XXX: this needs to get duplicaed in exactly the same
		// way in rhel/imagetype.go
		workloadRepos := payloadRepos
		customRepos, err := bp.Customizations.GetRepositories()
		if err != nil {
			return nil, nil, err
		}
		installFromRepos := blueprint.RepoCustomizationsInstallFromOnly(customRepos)
		workloadRepos = append(workloadRepos, installFromRepos...)

		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: workloadRepos,
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
	for idx, cont := range bp.Containers {
		containerSources[idx] = container.SourceSpec{
			Source:    cont.Source,
			Name:      cont.Name,
			TLSVerify: cont.TLSVerify,
			Local:     cont.LocalStorage,
		}
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	img, err := t.image(w, t, bp, options, staticPackageSets, containerSources, rng)
	if err != nil {
		return nil, nil, err
	}
	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
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

	var warnings []string

	if !t.rpmOstree && options.OSTree != nil {
		return warnings, fmt.Errorf("OSTree is not supported for %q", t.Name())
	}

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.rpmOstree && (t.name != "iot-commit" && t.name != "iot-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if t.bootISO && t.rpmOstree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
	}

	if t.name == "iot-raw-image" || t.name == "iot-qcow2-image" {
		allowed := []string{"User", "Group", "Directories", "Files", "Services", "FIPS"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.name, strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer" in RHEL distros
	}

	// BootISOs have limited support for customizations.
	// TODO: Support kernel name selection for image-installer
	if t.bootISO {
		if t.name == "iot-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "Ignition", "Kernel", "User", "Group", "FIPS"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.name, strings.Join(allowed, ", "))
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
		} else if t.name == "iot-installer" || t.name == "image-installer" {
			// "Installer" is actually not allowed for image-installer right now, but this is checked at the end
			allowed := []string{"User", "Group", "FIPS", "Installer", "Timezone", "Locale"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.name, strings.Join(allowed, ", "))
			}
		} else if t.name == "live-installer" {
			allowed := []string{"Installer"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.NoCustomizationsAllowedError, t.name)
			}
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return warnings, err
	}
	if (len(mountpoints) > 0 || partitioning != nil) && t.rpmOstree {
		return warnings, fmt.Errorf("Custom mountpoints and partitioning are not supported for ostree types")
	}
	if len(mountpoints) > 0 && partitioning != nil {
		return warnings, fmt.Errorf("partitioning customizations cannot be used with custom filesystems (mountpoints)")
	}

	if err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies); err != nil {
		return warnings, err
	}
	if err := blueprint.CheckDiskMountpointsPolicy(partitioning, policies.MountpointPolicies); err != nil {
		return warnings, err
	}
	if err := partitioning.ValidateLayoutConstraints(); err != nil {
		return nil, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		supported := oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList)
		if !supported {
			return warnings, fmt.Errorf("OpenSCAP unsupported profile: %s", osc.ProfileID)
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

	dcp := policies.CustomDirectoriesPolicies
	fcp := policies.CustomFilesPolicies

	if t.rpmOstree {
		dcp = policies.OstreeCustomDirectoriesPolicies
		fcp = policies.OstreeCustomFilesPolicies
	}

	err = blueprint.CheckDirectoryCustomizationsPolicy(dc, dcp)
	if err != nil {
		return warnings, err
	}

	err = blueprint.CheckFileCustomizationsPolicy(fc, fcp)
	if err != nil {
		return warnings, err
	}

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return warnings, err
	}

	if customizations.GetFIPS() && !common.IsBuildHostFIPSEnabled() {
		warnings = append(warnings, fmt.Sprintln(common.FIPSEnabledImageWarning))
	}

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return warnings, err
	}
	if instCust != nil {
		// only supported by the Anaconda installer
		if slices.Index([]string{"iot-installer"}, t.name) == -1 {
			return warnings, fmt.Errorf("installer customizations are not supported for %q", t.Name())
		}

		// NOTE: the image type check is redundant with the check above, but
		// let's keep it explicit in case one of the two changes.
		// The kickstart contents is incompatible with the users and groups
		// customization only for the iot-installer.
		if t.Name() == "iot-installer" &&
			instCust.Kickstart != nil &&
			len(instCust.Kickstart.Contents) > 0 &&
			(customizations.GetUsers() != nil || customizations.GetGroups() != nil) {
			return warnings, fmt.Errorf("iot-installer installer.kickstart.contents are not supported in combination with users or groups")
		}
	}

	return warnings, nil
}
