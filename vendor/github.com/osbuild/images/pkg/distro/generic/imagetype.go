package generic

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/policies"
	"github.com/osbuild/images/pkg/rpmmd"
)

type imageFunc func(workload workload.Workload, t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type isoLabelFunc func(t *imageType) string

// imageType implements the distro.ImageType interface
var _ = distro.ImageType(&imageType{})

type imageType struct {
	defs.ImageTypeYAML

	arch     *architecture
	platform platform.Platform

	// XXX: make definable via YAML
	workload               workload.Workload
	defaultImageConfig     *distro.ImageConfig
	defaultInstallerConfig *distro.InstallerConfig

	image    imageFunc
	isoLabel isoLabelFunc
}

func (t *imageType) Name() string {
	return t.ImageTypeYAML.Name()
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.ImageTypeYAML.Filename
}

func (t *imageType) MIMEType() string {
	return t.ImageTypeYAML.MimeType
}

func (t *imageType) OSTreeRef() string {
	d := t.arch.distro
	if t.ImageTypeYAML.RPMOSTree {
		return fmt.Sprintf(d.OSTreeRef(), t.arch.Name())
	}
	return ""
}

func (t *imageType) ISOLabel() (string, error) {
	if !t.ImageTypeYAML.BootISO {
		return "", fmt.Errorf("image type %q is not an ISO", t.Name())
	}

	if t.isoLabel != nil {
		return t.isoLabel(t), nil
	}

	return "", nil
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.ImageTypeYAML.Name() == "vhd" && size%datasizes.MebiByte != 0 {
		size = (size/datasizes.MebiByte + 1) * datasizes.MebiByte
	}
	if size == 0 {
		size = t.ImageTypeYAML.DefaultSize
	}
	return size
}

func (t *imageType) BuildPipelines() []string {
	return t.ImageTypeYAML.BuildPipelines
}

func (t *imageType) PayloadPipelines() []string {
	return t.ImageTypeYAML.PayloadPipelines
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) Exports() []string {
	if len(t.ImageTypeYAML.Exports) > 0 {
		return t.ImageTypeYAML.Exports
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

func (t *imageType) BasePartitionTable() (*disk.PartitionTable, error) {
	return t.ImageTypeYAML.PartitionTable(t.arch.distro.Name(), t.arch.name)
}

func (t *imageType) getPartitionTable(customizations *blueprint.Customizations, options distro.ImageOptions, rng *rand.Rand) (*disk.PartitionTable, error) {
	basePartitionTable, err := t.BasePartitionTable()
	if err != nil {
		return nil, err
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
			DefaultFSType:      t.arch.distro.DefaultFSType,
			RequiredMinSizes:   t.ImageTypeYAML.RequiredPartitionSizes,
			Architecture:       t.platform.GetArch(),
		}
		return disk.NewCustomPartitionTable(partitioning, partOptions, rng)
	}

	partitioningMode := options.PartitioningMode
	if t.ImageTypeYAML.RPMOSTree {
		// IoT supports only LVM, force it.
		// Raw is not supported, return an error if it is requested
		// TODO Need a central location for logic like this
		if partitioningMode == disk.RawPartitioningMode {
			return nil, fmt.Errorf("partitioning mode raw not supported for %s on %s", t.Name(), t.arch.Name())
		}
		partitioningMode = disk.AutoLVMPartitioningMode
	}

	mountpoints := customizations.GetFilesystems()
	return disk.NewPartitionTable(basePartitionTable, mountpoints, imageSize, partitioningMode, t.platform.GetArch(), t.ImageTypeYAML.RequiredPartitionSizes, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.defaultImageConfig)

}

func (t *imageType) getDefaultInstallerConfig() (*distro.InstallerConfig, error) {
	if !t.ImageTypeYAML.BootISO {
		return nil, fmt.Errorf("image type %q is not an ISO", t.Name())
	}

	return t.defaultInstallerConfig, nil
}

func (t *imageType) PartitionType() disk.PartitionTableType {
	basePartitionTable, err := t.BasePartitionTable()
	if errors.Is(err, defs.ErrNoPartitionTableForImgType) {
		return disk.PT_NONE
	}
	if err != nil {
		panic(err)
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
		pkgSets, err := t.ImageTypeYAML.PackageSets(t.arch.distro.Name(), t.arch.name)
		if err != nil {
			return nil, nil, err
		}
		for name, pkgSet := range pkgSets {
			staticPackageSets[name] = pkgSet
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
			Packages:       bp.GetPackagesEx(false),
			EnabledModules: bp.GetEnabledModules(),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
			cw.MaskedServices = services.Masked
		}
		w = cw
	}

	if experimentalflags.Bool("no-fstab") {
		if t.defaultImageConfig == nil {
			t.defaultImageConfig = &distro.ImageConfig{
				MountUnits: common.ToPtr(true),
			}
		} else {
			t.defaultImageConfig.MountUnits = common.ToPtr(true)
		}
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
	// TODO: remove the need for this entirely, the manifest has a
	// bunch of code that checks the distro currently, ideally all
	// would just be encoded in the YAML
	mf.Distro = t.arch.distro.DistroYAML.DistroLike
	if mf.Distro == manifest.DISTRO_NULL {
		return nil, nil, fmt.Errorf("no distro_like set in yaml for %q", t.arch.distro.Name())
	}
	if options.UseBootstrapContainer {
		mf.DistroBootstrapRef = bootstrapContainerFor(t)
	}
	_, err = img.InstantiateManifest(&mf, repos, &t.arch.distro.DistroYAML.Runner, rng)
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

	if !t.ImageTypeYAML.RPMOSTree && options.OSTree != nil {
		return warnings, fmt.Errorf("OSTree is not supported for %q", t.Name())
	}

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.ImageTypeYAML.RPMOSTree && (t.Name() != "iot-commit" && t.Name() != "iot-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.arch.distro.Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if t.ImageTypeYAML.BootISO && t.ImageTypeYAML.RPMOSTree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.Name())
		}
	}

	if t.Name() == "iot-raw-xz" || t.Name() == "iot-qcow2" {
		allowed := []string{"User", "Group", "Directories", "Files", "Services", "FIPS"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer" in RHEL distros
	}

	// BootISOs have limited support for customizations.
	// TODO: Support kernel name selection for image-installer
	if t.ImageTypeYAML.BootISO {
		if t.Name() == "iot-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "Ignition", "Kernel", "User", "Group", "FIPS"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return warnings, fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.Name())
			}

			// FDO is optional, but when specified has some restrictions
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to when using FDO", t.Name())
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
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to when using FDO", t.Name())
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
		} else if t.Name() == "iot-installer" || t.Name() == "minimal-installer" {
			// "Installer" is actually not allowed for image-installer right now, but this is checked at the end
			allowed := []string{"User", "Group", "FIPS", "Installer", "Timezone", "Locale"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
			}
		} else if t.Name() == "workstation-live-installer" {
			allowed := []string{"Installer"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.NoCustomizationsAllowedError, t.Name())
			}
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.ImageTypeYAML.RPMOSTree {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return warnings, err
	}
	if (len(mountpoints) > 0 || partitioning != nil) && t.ImageTypeYAML.RPMOSTree {
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
		supported := oscap.IsProfileAllowed(osc.ProfileID, t.arch.distro.DistroYAML.OscapProfilesAllowList)
		if !supported {
			return warnings, fmt.Errorf("OpenSCAP unsupported profile: %s", osc.ProfileID)
		}
		if t.ImageTypeYAML.RPMOSTree {
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

	if t.ImageTypeYAML.RPMOSTree {
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
		if !slices.Contains([]string{"image-installer", "edge-installer", "live-installer", "iot-installer"}, t.Name()) {
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

func bootstrapContainerFor(t *imageType) string {
	a := common.Must(arch.FromString(t.arch.name))
	return t.arch.distro.DistroYAML.BootstrapContainers[a]
}
