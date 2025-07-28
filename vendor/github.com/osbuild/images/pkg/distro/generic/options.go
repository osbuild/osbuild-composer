package generic

import (
	"fmt"
	"slices"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/policies"
)

func checkOptionsCommon(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	if !t.RPMOSTree && options.OSTree != nil {
		return nil, fmt.Errorf("OSTree is not supported for %q", t.Name())
	}

	if len(t.ImageTypeYAML.SupportedPartitioningModes) > 0 && !slices.Contains(t.ImageTypeYAML.SupportedPartitioningModes, options.PartitioningMode) {
		return nil, fmt.Errorf("partitioning mode %s not supported for %q", options.PartitioningMode, t.Name())
	}
	return nil, nil
}

func checkOptionsRhel10(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	customizations := bp.Customizations
	// holds warnings (e.g. deprecation notices)
	var warnings []string
	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}
	if err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies); err != nil {
		return warnings, err
	}
	if len(mountpoints) > 0 && partitioning != nil {
		return nil, fmt.Errorf("partitioning customizations cannot be used with custom filesystems (mountpoints)")
	}
	if err := blueprint.CheckDiskMountpointsPolicy(partitioning, policies.MountpointPolicies); err != nil {
		return warnings, err
	}
	if err := partitioning.ValidateLayoutConstraints(); err != nil {
		return warnings, err
	}
	if osc := customizations.GetOpenSCAP(); osc != nil {
		if !oscap.IsProfileAllowed(osc.ProfileID, t.arch.distro.DistroYAML.OscapProfilesAllowList) {
			return warnings, fmt.Errorf("OpenSCAP unsupported profile: %s", osc.ProfileID)
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
	if t.RPMOSTree {
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
		if slices.Index([]string{"image-installer", "edge-installer", "live-installer"}, t.Name()) == -1 {
			return warnings, fmt.Errorf("installer customizations are not supported for %q", t.Name())
		}
	}
	return warnings, nil
}
func checkOptionsRhel9(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {

	customizations := bp.Customizations

	// holds warnings (e.g. deprecation notices)
	var warnings []string

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.RPMOSTree && (t.Name() != "edge-commit" && t.Name() != "edge-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.Arch().Distro().Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if t.BootISO && t.RPMOSTree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.Name())
		}

		if t.Name() == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "Ignition", "Kernel", "User", "Group", "FIPS", "Filesystem"}
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
		} else if t.Name() == "edge-installer" {
			allowed := []string{"User", "Group", "FIPS", "Installer", "Timezone", "Locale"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
			}
		}
	}

	if t.Name() == "edge-raw-image" || t.Name() == "edge-ami" || t.Name() == "edge-vsphere" {
		// ostree-based bootable images require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("%q images require specifying a URL from which to retrieve the OSTree commit", t.Name())
		}

		allowed := []string{"Ignition", "Kernel", "User", "Group", "FIPS", "Filesystem"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer"
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.RPMOSTree && t.Name() != "edge-raw-image" && t.Name() != "edge-simplified-installer" {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}
	if (mountpoints != nil || partitioning != nil) && t.RPMOSTree && (t.Name() == "edge-container" || t.Name() == "edge-commit") {
		return warnings, fmt.Errorf("custom mountpoints and partitioning are not supported for ostree types")
	} else if (mountpoints != nil || partitioning != nil) && t.RPMOSTree && !(t.Name() == "edge-container" || t.Name() == "edge-commit") {
		//customization allowed for edge-raw-image,edge-ami,edge-vsphere,edge-simplified-installer
		err := blueprint.CheckMountpointsPolicy(mountpoints, policies.OstreeMountpointPolicies)
		if err != nil {
			return warnings, err
		}
	}

	if len(mountpoints) > 0 && partitioning != nil {
		return nil, fmt.Errorf("partitioning customizations cannot be used with custom filesystems (mountpoints)")
	}

	if err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies); err != nil {
		return warnings, err
	}

	if err := blueprint.CheckDiskMountpointsPolicy(partitioning, policies.MountpointPolicies); err != nil {
		return warnings, err
	}

	if err := partitioning.ValidateLayoutConstraints(); err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.Arch().Distro().OsVersion() == "9.0" {
			return warnings, fmt.Errorf("OpenSCAP unsupported os version: %s", t.Arch().Distro().OsVersion())
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, t.arch.distro.DistroYAML.OscapProfilesAllowList) {
			return warnings, fmt.Errorf("OpenSCAP unsupported profile: %s", osc.ProfileID)
		}
		if t.RPMOSTree {
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

	if t.RPMOSTree {
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
		if slices.Index([]string{"image-installer", "edge-installer", "live-installer"}, t.Name()) == -1 {
			return warnings, fmt.Errorf("installer customizations are not supported for %q", t.Name())
		}

		if t.Name() == "edge-installer" &&
			instCust.Kickstart != nil &&
			len(instCust.Kickstart.Contents) > 0 &&
			(customizations.GetUsers() != nil || customizations.GetGroups() != nil) {
			return warnings, fmt.Errorf("edge-installer installer.kickstart.contents are not supported in combination with users or groups")
		}
	}

	// don't support setting any kernel customizations for image types with
	// UKIs
	// NOTE: this is very ugly and stupid, it should not be based on the image
	// type name, but we want to redo this whole function anyway
	// NOTE: we can't use customizations.GetKernel() because it returns
	// 'Name: "kernel"' when unset.
	if t.Name() == "azure-cvm" && customizations != nil && customizations.Kernel != nil {
		return warnings, fmt.Errorf("kernel customizations are not supported for %q", t.Name())
	}

	return warnings, nil
}

func checkOptionsRhel8(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	customizations := bp.Customizations
	// holds warnings (e.g. deprecation notices)
	var warnings []string

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.RPMOSTree && (t.Name() != "edge-commit" && t.Name() != "edge-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.Arch().Distro().Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if t.BootISO && t.RPMOSTree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.Name())
		}

		if t.Name() == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "User", "Group", "FIPS"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return warnings, fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.Name())
			}
			//making fdo optional so that simplified installer can be composed w/o the FDO section in the blueprint
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to", t.Name())
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
					return warnings, fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to", t.Name())
				}
			}
		} else if t.Name() == "edge-installer" {
			allowed := []string{"User", "Group", "FIPS", "Installer", "Timezone", "Locale"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
			}
		}
	}

	if t.Name() == "edge-raw-image" {
		// ostree-based bootable images require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return warnings, fmt.Errorf("%q images require specifying a URL from which to retrieve the OSTree commit", t.Name())
		}

		allowed := []string{"User", "Group", "FIPS"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return warnings, fmt.Errorf(distro.UnsupportedCustomizationError, t.Name(), strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer"
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.RPMOSTree && t.Name() != "edge-raw-image" && t.Name() != "edge-simplified-installer" {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}

	if partitioning != nil {
		for _, partition := range partitioning.Partitions {
			if t.Arch().Name() == arch.ARCH_AARCH64.String() {
				if partition.FSType == "swap" {
					return warnings, fmt.Errorf("swap partition creation is not supported on %s %s", t.Arch().Distro().Name(), t.Arch().Name())
				}
				for _, lv := range partition.LogicalVolumes {
					if lv.FSType == "swap" {
						return warnings, fmt.Errorf("swap partition creation is not supported on %s %s", t.Arch().Distro().Name(), t.Arch().Name())
					}
				}
			}
		}
	}

	if mountpoints != nil && t.RPMOSTree {
		return warnings, fmt.Errorf("Custom mountpoints and partitioning are not supported for ostree types")
	}

	if err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies); err != nil {
		return warnings, err
	}

	if err := partitioning.ValidateLayoutConstraints(); err != nil {
		return warnings, err
	}

	if err := blueprint.CheckDiskMountpointsPolicy(partitioning, policies.MountpointPolicies); err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.Arch().Distro().OsVersion() == "9.0" {
			return warnings, fmt.Errorf("OpenSCAP unsupported os version: %s", t.Arch().Distro().OsVersion())
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, t.arch.distro.DistroYAML.OscapProfilesAllowList) {
			return warnings, fmt.Errorf("OpenSCAP unsupported profile: %s", osc.ProfileID)
		}
		if t.RPMOSTree {
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

	if t.RPMOSTree {
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
		w := fmt.Sprintln(common.FIPSEnabledImageWarning)
		warnings = append(warnings, w)
	}

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return warnings, err
	}
	if instCust != nil {
		// only supported by the Anaconda installer
		if slices.Index([]string{"image-installer", "edge-installer", "live-installer"}, t.Name()) == -1 {
			return warnings, fmt.Errorf("installer customizations are not supported for %q", t.Name())
		}

		if t.Name() == "edge-installer" &&
			instCust.Kickstart != nil &&
			len(instCust.Kickstart.Contents) > 0 &&
			(customizations.GetUsers() != nil || customizations.GetGroups() != nil) {
			return warnings, fmt.Errorf("edge-installer installer.kickstart.contents are not supported in combination with users or groups")
		}
	}

	return warnings, nil

}

func checkOptionsRhel7(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	customizations := bp.Customizations
	// holds warnings (e.g. deprecation notices)
	var warnings []string
	if len(bp.Containers) > 0 {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.Arch().Distro().Name())
	}
	mountpoints := customizations.GetFilesystems()
	err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies)
	if err != nil {
		return warnings, err
	}
	if osc := customizations.GetOpenSCAP(); osc != nil {
		return warnings, fmt.Errorf("OpenSCAP unsupported os version: %s", t.Arch().Distro().OsVersion())
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
	return warnings, nil
}

func checkOptionsFedora(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	customizations := bp.Customizations

	var warnings []string

	if !t.RPMOSTree && options.OSTree != nil {
		return warnings, fmt.Errorf("OSTree is not supported for %q", t.Name())
	}

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.RPMOSTree && (t.Name() != "iot-commit" && t.Name() != "iot-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.arch.distro.Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if t.BootISO && t.RPMOSTree {
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
	if t.BootISO {
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

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.RPMOSTree {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return warnings, err
	}
	if (len(mountpoints) > 0 || partitioning != nil) && t.RPMOSTree {
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
		if t.RPMOSTree {
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

	if t.RPMOSTree {
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
		if slices.Index([]string{"iot-installer"}, t.Name()) == -1 {
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
