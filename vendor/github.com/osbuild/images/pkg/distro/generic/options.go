package generic

import (
	"fmt"
	"slices"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
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

	customizations := bp.Customizations

	var warnings []string

	errPrefix := fmt.Sprintf("blueprint validation failed for image type %q", t.Name())

	if err := distro.ValidateConfig(t, *bp); err != nil {
		// NOTE (validation-warnings): appending to warnings now, because this
		// is breaking a lot of things the service
		errAsWarning := fmt.Errorf("%s: %w", errPrefix, err)
		warnings = append(warnings, errAsWarning.Error())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return warnings, err
		}
	}

	if (t.BootISO || t.Bootable) && t.RPMOSTree {
		// ostree-based ISOs require a URL from which to pull a payload commit, this can either be a default URL or one
		// supplied through options
		if t.OSTreeURL() == "" && (options.OSTree == nil || options.OSTree.URL == "") {
			return warnings, fmt.Errorf("options validation failed for image type %q: ostree.url: required, there is no default available", t.Name())
		}
	}

	// FDO is optional, but when specified has some restrictions
	if customizations.GetFDO() != nil {
		if customizations.GetFDO().ManufacturingServerURL == "" {
			return warnings, fmt.Errorf("%s: customizations.fdo.manufacturing_server_url: required when using fdo", errPrefix)
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
			return warnings, fmt.Errorf("%s: exactly one of customizations.fdo.diun_pub_key_hash, customizations.fdo.diun_pub_key_insecure, customizations.fdo.diun_pub_key_root_certs: required when using fdo", errPrefix)
		}
	}

	if customizations.GetIgnition() != nil {
		if customizations.GetIgnition().Embedded != nil && customizations.GetIgnition().FirstBoot != nil {
			return warnings, fmt.Errorf("%s: customizations.ignition.embedded cannot be used with customizations.ignition.firstboot", errPrefix)
		}
		if customizations.GetIgnition().FirstBoot != nil && customizations.GetIgnition().FirstBoot.ProvisioningURL == "" {
			return warnings, fmt.Errorf("%s: customizations.ignition.firstboot requires customizations.ignition.firstboot.provisioning_url", errPrefix)
		}
	}

	mountpoints := customizations.GetFilesystems()
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return warnings, err
	}
	if len(mountpoints) > 0 && partitioning != nil {
		return warnings, fmt.Errorf("%s: customizations.disk cannot be used with customizations.filesystem", errPrefix)
	}

	if err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies); err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}
	if err := blueprint.CheckDiskMountpointsPolicy(partitioning, policies.MountpointPolicies); err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}
	if err := partitioning.ValidateLayoutConstraints(); err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		d := t.arch.distro.(*distribution)
		supported := oscap.IsProfileAllowed(osc.ProfileID, d.DistroYAML.OscapProfilesAllowList)
		if !supported {
			return warnings, fmt.Errorf("%s: customizations.openscap.profile_id: unsupported profile %s", errPrefix, osc.ProfileID)
		}
		if osc.ProfileID == "" {
			return warnings, fmt.Errorf("%s: customizations.openscap.profile_id: required when using customizations.openscap", errPrefix)
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
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}

	err = blueprint.CheckFileCustomizationsPolicy(fc, fcp)
	if err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}

	if customizations.GetFIPS() && !common.IsBuildHostFIPSEnabled() {
		warnings = append(warnings, fmt.Sprintln(common.FIPSEnabledImageWarning))
	}

	// check if group customizations are valid
	if _, err := customizations.GetGroups(); err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return warnings, fmt.Errorf("%s: %w", errPrefix, err)
	}
	if instCust != nil && instCust.Kickstart != nil && len(instCust.Kickstart.Contents) > 0 {
		groups, _ := customizations.GetGroups() // group customization errors are checked above
		if customizations.GetUsers() != nil || groups != nil {
			return warnings, fmt.Errorf("%s: customizations.installer.kickstart.contents cannot be used with customizations.user or customizations.group", errPrefix)
		}
	}
	return warnings, nil
}

func checkOptionsRhel9(t *imageType, bp *blueprint.Blueprint) error {
	customizations := bp.Customizations
	errPrefix := fmt.Sprintf("blueprint validation failed for image type %q", t.Name())

	if osc := customizations.GetOpenSCAP(); osc != nil {
		// TODO: remove this check when we add support for conditions in
		// supported_blueprint_options.
		if t.Arch().Distro().OsVersion() == "9.0" {
			return fmt.Errorf("%s: customizations.openscap: not supported for distro version: %s", errPrefix, t.Arch().Distro().OsVersion())
		}
	}
	return nil
}

func checkOptionsRhel8(t *imageType, bp *blueprint.Blueprint) error {
	customizations := bp.Customizations

	errPrefix := fmt.Sprintf("blueprint validation failed for image type %q", t.Name())

	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return err
	}

	if partitioning != nil {
		for _, partition := range partitioning.Partitions {
			if t.Arch().Name() == arch.ARCH_AARCH64.String() {
				// Due to kernel page size differences, between RHEL 8 and
				// newer distros, it is impossible to create a swap partition
				// for RHEL 8. Disk customizations and swap creation were only
				// added in RHEL 9, so there is no case where this could be
				// running on RHEL 8, and the RHEL 9 aarch64 kernel uses a
				// different page size.
				if partition.FSType == "swap" {
					return fmt.Errorf("%s: customizations.disk: swap partition creation is not supported on %s %s", errPrefix, t.Arch().Distro().Name(), t.Arch().Name())
				}
				for _, lv := range partition.LogicalVolumes {
					if lv.FSType == "swap" {
						return fmt.Errorf("%s: customizations.disk: swap logical volume creation is not supported on %s %s", errPrefix, t.Arch().Distro().Name(), t.Arch().Name())
					}
				}
			}
		}
	}
	if osc := customizations.GetOpenSCAP(); osc != nil {
		if osVersion := t.Arch().Distro().OsVersion(); common.VersionLessThan(osVersion, "8.7") {
			return fmt.Errorf("%s: customizations.openscap: not supported for distro version: %s", errPrefix, osVersion)
		}
	}

	return nil
}
