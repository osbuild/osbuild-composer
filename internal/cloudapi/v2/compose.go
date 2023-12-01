package v2

// ComposeRequest methods to make it easier to use and test
import (
	"fmt"
	"reflect"

	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/subscription"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
)

// GetBlueprintWithCustomizations returns a new Blueprint with all of the
// customizations set from the ComposeRequest
func (request *ComposeRequest) GetBlueprintWithCustomizations() (blueprint.Blueprint, error) {
	var bp = blueprint.Blueprint{Name: "empty blueprint"}
	err := bp.Initialize()
	if err != nil {
		return bp, HTTPErrorWithInternal(ErrorFailedToInitializeBlueprint, err)
	}

	if request.Customizations == nil {
		return bp, nil
	}

	// Assume there is going to be one or more customization
	bp.Customizations = &blueprint.Customizations{}

	// Set the blueprint customisation to take care of the user
	if request.Customizations.Users != nil {
		var userCustomizations []blueprint.UserCustomization
		for _, user := range *request.Customizations.Users {
			var groups []string
			if user.Groups != nil {
				groups = *user.Groups
			} else {
				groups = nil
			}
			userCustomizations = append(userCustomizations,
				blueprint.UserCustomization{
					Name:   user.Name,
					Key:    user.Key,
					Groups: groups,
				},
			)
		}
		bp.Customizations.User = userCustomizations
	}

	if request.Customizations.Packages != nil {
		for _, p := range *request.Customizations.Packages {
			bp.Packages = append(bp.Packages, blueprint.Package{
				Name: p,
			})
		}
	}

	if request.Customizations.Containers != nil {
		for _, c := range *request.Customizations.Containers {
			bc := blueprint.Container{
				Source:    c.Source,
				TLSVerify: c.TlsVerify,
			}
			if c.Name != nil {
				bc.Name = *c.Name
			}
			bp.Containers = append(bp.Containers, bc)
		}
	}

	if request.Customizations.Directories != nil {
		var dirCustomizations []blueprint.DirectoryCustomization
		for _, d := range *request.Customizations.Directories {
			dirCustomization := blueprint.DirectoryCustomization{
				Path: d.Path,
			}
			if d.Mode != nil {
				dirCustomization.Mode = *d.Mode
			}
			if d.User != nil {
				dirCustomization.User = *d.User
				if uid, ok := dirCustomization.User.(float64); ok {
					// check if uid can be converted to int64
					if uid != float64(int64(uid)) {
						return bp, fmt.Errorf("invalid user %f: must be an integer", uid)
					}
					dirCustomization.User = int64(uid)
				}
			}
			if d.Group != nil {
				dirCustomization.Group = *d.Group
				if gid, ok := dirCustomization.Group.(float64); ok {
					// check if gid can be converted to int64
					if gid != float64(int64(gid)) {
						return bp, fmt.Errorf("invalid group %f: must be an integer", gid)
					}
					dirCustomization.Group = int64(gid)
				}
			}
			if d.EnsureParents != nil {
				dirCustomization.EnsureParents = *d.EnsureParents
			}
			dirCustomizations = append(dirCustomizations, dirCustomization)
		}

		// Validate the directory customizations, because the Cloud API does not use the custom unmarshaller
		_, err := blueprint.DirectoryCustomizationsToFsNodeDirectories(dirCustomizations)
		if err != nil {
			return bp, HTTPErrorWithInternal(ErrorInvalidCustomization, err)
		}

		bp.Customizations.Directories = dirCustomizations
	}

	if request.Customizations.Files != nil {
		var fileCustomizations []blueprint.FileCustomization
		for _, f := range *request.Customizations.Files {
			fileCustomization := blueprint.FileCustomization{
				Path: f.Path,
			}
			if f.Data != nil {
				fileCustomization.Data = *f.Data
			}
			if f.Mode != nil {
				fileCustomization.Mode = *f.Mode
			}
			if f.User != nil {
				fileCustomization.User = *f.User
				if uid, ok := fileCustomization.User.(float64); ok {
					// check if uid can be converted to int64
					if uid != float64(int64(uid)) {
						return bp, fmt.Errorf("invalid user %f: must be an integer", uid)
					}
					fileCustomization.User = int64(uid)
				}
			}
			if f.Group != nil {
				fileCustomization.Group = *f.Group
				if gid, ok := fileCustomization.Group.(float64); ok {
					// check if gid can be converted to int64
					if gid != float64(int64(gid)) {
						return bp, fmt.Errorf("invalid group %f: must be an integer", gid)
					}
					fileCustomization.Group = int64(gid)
				}
			}
			fileCustomizations = append(fileCustomizations, fileCustomization)
		}

		// Validate the file customizations, because the Cloud API does not use the custom unmarshaller
		_, err := blueprint.FileCustomizationsToFsNodeFiles(fileCustomizations)
		if err != nil {
			return bp, HTTPErrorWithInternal(ErrorInvalidCustomization, err)
		}

		bp.Customizations.Files = fileCustomizations
	}

	if request.Customizations.Filesystem != nil {
		var fsCustomizations []blueprint.FilesystemCustomization
		for _, f := range *request.Customizations.Filesystem {

			fsCustomizations = append(fsCustomizations,
				blueprint.FilesystemCustomization{
					Mountpoint: f.Mountpoint,
					MinSize:    f.MinSize,
				},
			)
		}
		bp.Customizations.Filesystem = fsCustomizations
	}

	if request.Customizations.Services != nil {
		servicesCustomization := &blueprint.ServicesCustomization{}
		if request.Customizations.Services.Enabled != nil {
			servicesCustomization.Enabled = make([]string, len(*request.Customizations.Services.Enabled))
			copy(servicesCustomization.Enabled, *request.Customizations.Services.Enabled)
		}
		if request.Customizations.Services.Disabled != nil {
			servicesCustomization.Disabled = make([]string, len(*request.Customizations.Services.Disabled))
			copy(servicesCustomization.Disabled, *request.Customizations.Services.Disabled)
		}
		bp.Customizations.Services = servicesCustomization
	}

	if request.Customizations.Openscap != nil {
		openSCAPCustomization := &blueprint.OpenSCAPCustomization{
			ProfileID: request.Customizations.Openscap.ProfileId,
		}
		if tailoring := request.Customizations.Openscap.Tailoring; tailoring != nil {
			tailoringCustomizations := blueprint.OpenSCAPTailoringCustomizations{}
			if tailoring.Selected != nil && len(*tailoring.Selected) > 0 {
				tailoringCustomizations.Selected = *tailoring.Selected
			}
			if tailoring.Unselected != nil && len(*tailoring.Unselected) > 0 {
				tailoringCustomizations.Unselected = *tailoring.Unselected
			}
			openSCAPCustomization.Tailoring = &tailoringCustomizations
		}
		bp.Customizations.OpenSCAP = openSCAPCustomization
	}

	if request.Customizations.CustomRepositories != nil {
		repoCustomizations := []blueprint.RepositoryCustomization{}
		for _, repo := range *request.Customizations.CustomRepositories {
			repoCustomization := blueprint.RepositoryCustomization{
				Id: repo.Id,
			}

			if repo.Name != nil {
				repoCustomization.Name = *repo.Name
			}

			if repo.Filename != nil {
				repoCustomization.Filename = *repo.Filename
			}

			if repo.Baseurl != nil && len(*repo.Baseurl) > 0 {
				repoCustomization.BaseURLs = *repo.Baseurl
			}

			if repo.Gpgkey != nil && len(*repo.Gpgkey) > 0 {
				repoCustomization.GPGKeys = *repo.Gpgkey
			}

			if repo.CheckGpg != nil {
				repoCustomization.GPGCheck = repo.CheckGpg
			}

			if repo.CheckRepoGpg != nil {
				repoCustomization.RepoGPGCheck = repo.CheckRepoGpg
			}

			if repo.Enabled != nil {
				repoCustomization.Enabled = repo.Enabled
			}

			if repo.Metalink != nil {
				repoCustomization.Metalink = *repo.Metalink
			}

			if repo.Mirrorlist != nil {
				repoCustomization.Mirrorlist = *repo.Mirrorlist
			}

			if repo.SslVerify != nil {
				repoCustomization.SSLVerify = repo.SslVerify
			}

			if repo.Priority != nil {
				repoCustomization.Priority = repo.Priority
			}

			if repo.ModuleHotfixes != nil {
				repoCustomization.ModuleHotfixes = repo.ModuleHotfixes
			}

			repoCustomizations = append(repoCustomizations, repoCustomization)
		}
		bp.Customizations.Repositories = repoCustomizations
	}

	if request.Customizations.Hostname != nil {
		bp.Customizations.Hostname = request.Customizations.Hostname
	}

	if request.Customizations.Kernel != nil {
		kernel := &blueprint.KernelCustomization{}
		if request.Customizations.Kernel.Name != nil {
			kernel.Name = *request.Customizations.Kernel.Name
		}
		if request.Customizations.Kernel.Append != nil {
			kernel.Append = *request.Customizations.Kernel.Append
		}

		bp.Customizations.Kernel = kernel
	}

	if request.Customizations.Groups != nil {
		groups := []blueprint.GroupCustomization{}
		for _, group := range *request.Customizations.Groups {
			groups = append(groups, blueprint.GroupCustomization{
				Name: group.Name,
				GID:  group.Gid,
			})
		}

		bp.Customizations.Group = groups
	}

	if request.Customizations.Timezone != nil {
		tz := &blueprint.TimezoneCustomization{
			Timezone: request.Customizations.Timezone.Timezone,
		}

		if request.Customizations.Timezone.Ntpservers != nil {
			tz.NTPServers = append(tz.NTPServers, *request.Customizations.Timezone.Ntpservers...)
		}

		bp.Customizations.Timezone = tz
	}

	if request.Customizations.Locale != nil {
		locale := &blueprint.LocaleCustomization{
			Keyboard: request.Customizations.Locale.Keyboard,
		}

		if request.Customizations.Locale.Languages != nil {
			locale.Languages = append(locale.Languages, *request.Customizations.Locale.Languages...)
		}

		bp.Customizations.Locale = locale
	}

	if request.Customizations.Firewall != nil {
		firewall := &blueprint.FirewallCustomization{}

		if request.Customizations.Firewall.Ports != nil {
			firewall.Ports = append(firewall.Ports, *request.Customizations.Firewall.Ports...)
		}
		if request.Customizations.Firewall.Services != nil {
			enabled := []string{}
			if request.Customizations.Firewall.Services.Enabled != nil {
				enabled = append(enabled, *request.Customizations.Firewall.Services.Enabled...)
			}
			disabled := []string{}
			if request.Customizations.Firewall.Services.Disabled != nil {
				disabled = append(disabled, *request.Customizations.Firewall.Services.Disabled...)
			}
			firewall.Services = &blueprint.FirewallServicesCustomization{
				Enabled:  enabled,
				Disabled: disabled,
			}
		}

		bp.Customizations.Firewall = firewall
	}

	if request.Customizations.InstallationDevice != nil {
		if bp.Customizations == nil {
			bp.Customizations = &blueprint.Customizations{
				InstallationDevice: *request.Customizations.InstallationDevice,
			}
		} else {
			bp.Customizations.InstallationDevice = *request.Customizations.InstallationDevice
		}
	}

	if request.Customizations.Fdo != nil {
		fdo := &blueprint.FDOCustomization{}
		if request.Customizations.Fdo.DiunPubKeyHash != nil {
			fdo.DiunPubKeyHash = *request.Customizations.Fdo.DiunPubKeyHash
		}
		if request.Customizations.Fdo.DiunPubKeyInsecure != nil {
			fdo.DiunPubKeyInsecure = *request.Customizations.Fdo.DiunPubKeyInsecure
		}
		if request.Customizations.Fdo.DiunPubKeyRootCerts != nil {
			fdo.DiunPubKeyRootCerts = *request.Customizations.Fdo.DiunPubKeyRootCerts
		}
		if request.Customizations.Fdo.ManufacturingServerUrl != nil {
			fdo.ManufacturingServerURL = *request.Customizations.Fdo.ManufacturingServerUrl
		}
		if request.Customizations.Fdo.DiMfgStringTypeMacIface != nil {
			fdo.DiMfgStringTypeMacIface = *request.Customizations.Fdo.DiMfgStringTypeMacIface
		}

		bp.Customizations.FDO = fdo
	}

	if request.Customizations.Ignition != nil {
		ignition := &blueprint.IgnitionCustomization{}
		if request.Customizations.Ignition.Embedded != nil {
			ignition.Embedded = &blueprint.EmbeddedIgnitionCustomization{
				Config: request.Customizations.Ignition.Embedded.Config,
			}
		}
		if request.Customizations.Ignition.Firstboot != nil {
			ignition.FirstBoot = &blueprint.FirstBootIgnitionCustomization{
				ProvisioningURL: request.Customizations.Ignition.Firstboot.Url,
			}
		}
		bp.Customizations.Ignition = ignition
	}

	if request.Customizations.Fips != nil {
		bp.Customizations.FIPS = request.Customizations.Fips.Enabled
	}

	// Did bp.Customizations get set at all? If not, set it back to nil
	if reflect.DeepEqual(*bp.Customizations, blueprint.Customizations{}) {
		bp.Customizations = nil
	}

	return bp, nil
}

// GetPayloadRepositories returns the custom repos
// If there are none it returns a nil slice
func (request *ComposeRequest) GetPayloadRepositories() (repos []Repository) {
	if request.Customizations != nil && request.Customizations.PayloadRepositories != nil {
		repos = *request.Customizations.PayloadRepositories
	}

	return
}

// GetSubscription returns an ImageOptions struct populated by the subscription information
// included in the request, or nil if it has not been included.
func (request *ComposeRequest) GetSubscription() (sub *subscription.ImageOptions) {
	if request.Customizations != nil && request.Customizations.Subscription != nil {
		// Rhc is optional, default to false if not included
		var rhc bool
		if request.Customizations.Subscription.Rhc != nil {
			rhc = *request.Customizations.Subscription.Rhc
		}
		sub = &subscription.ImageOptions{
			Organization:  request.Customizations.Subscription.Organization,
			ActivationKey: request.Customizations.Subscription.ActivationKey,
			ServerUrl:     request.Customizations.Subscription.ServerUrl,
			BaseUrl:       request.Customizations.Subscription.BaseUrl,
			Insights:      request.Customizations.Subscription.Insights,
			Rhc:           rhc,
		}
	}

	return
}

// GetPartitioningMode returns the partitioning mode included in the request
// or defaults to AutoLVMPartitioningMode if not included
func (request *ComposeRequest) GetPartitioningMode() (disk.PartitioningMode, error) {
	if request.Customizations == nil || request.Customizations.PartitioningMode == nil {
		return disk.AutoLVMPartitioningMode, nil
	}

	switch *request.Customizations.PartitioningMode {
	case CustomizationsPartitioningModeRaw:
		return disk.RawPartitioningMode, nil
	case CustomizationsPartitioningModeLvm:
		return disk.LVMPartitioningMode, nil
	case CustomizationsPartitioningModeAutoLvm:
		return disk.AutoLVMPartitioningMode, nil
	}

	return disk.AutoLVMPartitioningMode, HTTPError(ErrorInvalidPartitioningMode)
}
