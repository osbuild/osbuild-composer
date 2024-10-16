package rhel9

import (
	"fmt"
	"log"
	"strings"

	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/policies"
)

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func checkOptions(t *rhel.ImageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {

	customizations := bp.Customizations

	// holds warnings (e.g. deprecation notices)
	var warnings []string

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.RPMOSTree && (t.Name() != "edge-commit" && t.Name() != "edge-container") {
		return warnings, fmt.Errorf("embedding containers is not supported for %s on %s", t.Name(), t.Arch().Distro().Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return nil, err
		}
	}

	if t.BootISO && t.RPMOSTree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return nil, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.Name())
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

	// warn that user & group customizations on edge-commit, edge-container are deprecated
	// TODO(edge): directly error if these options are provided when rhel-9.5's time arrives
	if t.Name() == "edge-commit" || t.Name() == "edge-container" {
		if customizations.GetUsers() != nil {
			w := fmt.Sprintf("Please note that user customizations on %q image type are deprecated and will be removed in the near future\n", t.Name())
			log.Print(w)
			warnings = append(warnings, w)
		}
		if customizations.GetGroups() != nil {
			w := fmt.Sprintf("Please note that group customizations on %q image type are deprecated and will be removed in the near future\n", t.Name())
			log.Print(w)
			warnings = append(warnings, w)
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.RPMOSTree && t.Name() != "edge-raw-image" && t.Name() != "edge-simplified-installer" {
		return warnings, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	if slices.Contains(t.UnsupportedPartitioningModes, options.PartitioningMode) {
		return warnings, fmt.Errorf("partitioning mode %q is not supported for %q", options.PartitioningMode, t.Name())
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.RPMOSTree && (t.Name() == "edge-container" || t.Name() == "edge-commit") {
		return warnings, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	} else if mountpoints != nil && t.RPMOSTree && !(t.Name() == "edge-container" || t.Name() == "edge-commit") {
		//customization allowed for edge-raw-image,edge-ami,edge-vsphere,edge-simplified-installer
		err := blueprint.CheckMountpointsPolicy(mountpoints, policies.OstreeMountpointPolicies)
		if err != nil {
			return warnings, err
		}
	}

	err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies)
	if err != nil {
		return warnings, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.Arch().Distro().OsVersion() == "9.0" {
			return warnings, fmt.Errorf("OpenSCAP unsupported os version: %s", t.Arch().Distro().OsVersion())
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList) {
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
		log.Print(w)
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
