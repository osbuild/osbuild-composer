package v2

// ComposeRequest methods to make it easier to use and test
import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/google/uuid"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

// Return the string representation of the partitioning mode
// default to auto-lvm (should never happen)
func (bcpm BlueprintCustomizationsPartitioningMode) String() string {
	switch bcpm {
	case BlueprintCustomizationsPartitioningModeAutoLvm:
		return "auto-lvm"
	case BlueprintCustomizationsPartitioningModeLvm:
		return "lvm"
	case BlueprintCustomizationsPartitioningModeRaw:
		return "raw"
	default:
		return "auto-lvm"
	}
}

// GetCustomizationsFromBlueprint populates a blueprint customization struct
// with the data from request Blueprint, which is similar but
// slightly different from the Cloudapi's Customizations section
// This starts with a new empty blueprint.Customization object
// If there are no customizations, it returns nil
func (rbp *Blueprint) GetCustomizationsFromBlueprintRequest() (*blueprint.Customizations, error) {
	if rbp.Customizations == nil {
		return nil, nil
	}

	c := &blueprint.Customizations{}
	rbpc := rbp.Customizations

	if rbpc.Hostname != nil {
		c.Hostname = rbpc.Hostname
	}

	if rbpc.Kernel != nil {
		kernel := &blueprint.KernelCustomization{}
		if rbpc.Kernel.Name != nil {
			kernel.Name = *rbpc.Kernel.Name
		}
		if rbpc.Kernel.Append != nil {
			kernel.Append = *rbpc.Kernel.Append
		}

		c.Kernel = kernel
	}

	if rbpc.Sshkey != nil {
		keys := []blueprint.SSHKeyCustomization{}
		for _, key := range *rbpc.Sshkey {
			keys = append(keys, blueprint.SSHKeyCustomization{
				User: key.User,
				Key:  key.Key,
			})
		}
		c.SSHKey = keys
	}

	if rbpc.User != nil {
		var userCustomizations []blueprint.UserCustomization
		for _, user := range *rbpc.User {
			uc := blueprint.UserCustomization{
				Name:        user.Name,
				Description: user.Description,
				Password:    user.Password,
				Key:         user.Key,
				Home:        user.Home,
				Shell:       user.Shell,
				UID:         user.Uid,
				GID:         user.Gid,
			}
			if user.Groups != nil {
				uc.Groups = append(uc.Groups, *user.Groups...)
			}
			userCustomizations = append(userCustomizations, uc)
		}
		c.User = userCustomizations
	}

	if rbpc.Group != nil {
		var groupCustomizations []blueprint.GroupCustomization
		for _, group := range *rbpc.Group {
			gc := blueprint.GroupCustomization{
				Name: group.Name,
				GID:  group.Gid,
			}
			groupCustomizations = append(groupCustomizations, gc)
		}
		c.Group = groupCustomizations

	}

	if rbpc.Timezone != nil {
		tz := &blueprint.TimezoneCustomization{
			Timezone: rbpc.Timezone.Timezone,
		}

		if rbpc.Timezone.Ntpservers != nil {
			tz.NTPServers = append(tz.NTPServers, *rbpc.Timezone.Ntpservers...)
		}

		c.Timezone = tz
	}

	if rbpc.Locale != nil {
		locale := &blueprint.LocaleCustomization{
			Keyboard: rbpc.Locale.Keyboard,
		}

		if rbpc.Locale.Languages != nil {
			locale.Languages = append(locale.Languages, *rbpc.Locale.Languages...)
		}

		c.Locale = locale
	}

	if rbpc.Firewall != nil {
		firewall := &blueprint.FirewallCustomization{}
		if rbpc.Firewall.Ports != nil {
			firewall.Ports = append(firewall.Ports, *rbpc.Firewall.Ports...)
		}
		if rbpc.Firewall.Services != nil {
			enabled := []string{}
			if rbpc.Firewall.Services.Enabled != nil {
				enabled = append(enabled, *rbpc.Firewall.Services.Enabled...)
			}
			disabled := []string{}
			if rbpc.Firewall.Services.Disabled != nil {
				disabled = append(disabled, *rbpc.Firewall.Services.Disabled...)
			}
			firewall.Services = &blueprint.FirewallServicesCustomization{
				Enabled:  enabled,
				Disabled: disabled,
			}
		}
		if rbpc.Firewall.Zones != nil {
			var zones []blueprint.FirewallZoneCustomization
			for _, zone := range *rbpc.Firewall.Zones {
				zc := blueprint.FirewallZoneCustomization{}
				if zone.Name != nil {
					zc.Name = zone.Name
				}
				if zone.Sources != nil {
					zc.Sources = append(zc.Sources, *zone.Sources...)
				}
				zones = append(zones, zc)
			}
			firewall.Zones = zones
		}

		c.Firewall = firewall
	}

	if rbpc.Services != nil {
		servicesCustomization := &blueprint.ServicesCustomization{}
		if rbpc.Services.Enabled != nil {
			servicesCustomization.Enabled = make([]string, len(*rbpc.Services.Enabled))
			copy(servicesCustomization.Enabled, *rbpc.Services.Enabled)
		}
		if rbpc.Services.Disabled != nil {
			servicesCustomization.Disabled = make([]string, len(*rbpc.Services.Disabled))
			copy(servicesCustomization.Disabled, *rbpc.Services.Disabled)
		}
		if rbpc.Services.Masked != nil {
			servicesCustomization.Masked = make([]string, len(*rbpc.Services.Masked))
			copy(servicesCustomization.Masked, *rbpc.Services.Masked)
		}
		c.Services = servicesCustomization
	}

	if rbpc.Filesystem != nil {
		var fsCustomizations []blueprint.FilesystemCustomization
		for _, f := range *rbpc.Filesystem {
			minSize, err := decodeMinsize(&f.Minsize)
			if err != nil {
				return nil, err
			}
			fsCustomizations = append(fsCustomizations,
				blueprint.FilesystemCustomization{
					Mountpoint: f.Mountpoint,
					MinSize:    minSize,
				},
			)
		}
		c.Filesystem = fsCustomizations
	}

	if rbpc.InstallationDevice != nil {
		c.InstallationDevice = *rbpc.InstallationDevice
	}

	if rbpc.PartitioningMode != nil {
		c.PartitioningMode = string(*rbpc.PartitioningMode)
	}

	if rbpc.Fdo != nil {
		fdo := &blueprint.FDOCustomization{}
		if rbpc.Fdo.DiunPubKeyHash != nil {
			fdo.DiunPubKeyHash = *rbpc.Fdo.DiunPubKeyHash
		}
		if rbpc.Fdo.DiunPubKeyInsecure != nil {
			fdo.DiunPubKeyInsecure = *rbpc.Fdo.DiunPubKeyInsecure
		}
		if rbpc.Fdo.DiunPubKeyRootCerts != nil {
			fdo.DiunPubKeyRootCerts = *rbpc.Fdo.DiunPubKeyRootCerts
		}
		if rbpc.Fdo.DiMfgStringTypeMacIface != nil {
			fdo.DiMfgStringTypeMacIface = *rbpc.Fdo.DiMfgStringTypeMacIface
		}
		if rbpc.Fdo.ManufacturingServerUrl != nil {
			fdo.ManufacturingServerURL = *rbpc.Fdo.ManufacturingServerUrl
		}

		c.FDO = fdo
	}

	if rbpc.Openscap != nil {
		oscap := &blueprint.OpenSCAPCustomization{
			ProfileID: rbpc.Openscap.ProfileId,
		}

		if rbpc.Openscap.PolicyId != nil {
			oscap.PolicyID = rbpc.Openscap.PolicyId.String()
		}

		if rbpc.Openscap.Datastream != nil {
			oscap.DataStream = *rbpc.Openscap.Datastream
		}
		if rbpc.Openscap.Tailoring != nil && rbpc.Openscap.JsonTailoring != nil {
			return nil, fmt.Errorf("OpenSCAP customization error: choose one option between OpenSCAP tailoring and OpenSCAP json tailoring")
		}
		if tailoring := rbpc.Openscap.Tailoring; tailoring != nil {
			tc := blueprint.OpenSCAPTailoringCustomizations{}
			if tailoring.Selected != nil && len(*tailoring.Selected) > 0 {
				tc.Selected = append(tc.Selected, *tailoring.Selected...)
			}
			if tailoring.Unselected != nil && len(*tailoring.Unselected) > 0 {
				tc.Unselected = append(tc.Unselected, *tailoring.Unselected...)
			}
			oscap.Tailoring = &tc
		}
		if jsonTailoring := rbpc.Openscap.JsonTailoring; jsonTailoring != nil {
			oscap.JSONTailoring = &blueprint.OpenSCAPJSONTailoringCustomizations{
				ProfileID: jsonTailoring.ProfileId,
				Filepath:  jsonTailoring.Filepath,
			}
		}
		c.OpenSCAP = oscap
	}

	if rbpc.Ignition != nil {
		ignition := &blueprint.IgnitionCustomization{}
		if rbpc.Ignition.Embedded != nil {
			ignition.Embedded = &blueprint.EmbeddedIgnitionCustomization{
				Config: rbpc.Ignition.Embedded.Config,
			}
		}
		if rbpc.Ignition.Firstboot != nil {
			ignition.FirstBoot = &blueprint.FirstBootIgnitionCustomization{
				ProvisioningURL: rbpc.Ignition.Firstboot.Url,
			}
		}
		c.Ignition = ignition
	}

	if rbpc.Directories != nil {
		var dirCustomizations []blueprint.DirectoryCustomization
		for _, d := range *rbpc.Directories {
			dirCustomization := blueprint.DirectoryCustomization{
				Path: d.Path,
			}
			if d.Mode != nil {
				dirCustomization.Mode = *d.Mode
			}
			if d.User != nil {
				user0, err := d.User.AsDirectoryUser0()
				if err == nil {
					dirCustomization.User = user0
				} else {
					user1, err := d.User.AsDirectoryUser1()
					if err != nil {
						return nil, fmt.Errorf("invalid user: %w", err)
					}
					dirCustomization.User = user1
				}
			}
			if d.Group != nil {
				group0, err := d.Group.AsDirectoryGroup0()
				if err == nil {
					dirCustomization.Group = group0
				} else {
					group1, err := d.Group.AsDirectoryGroup1()
					if err != nil {
						return nil, fmt.Errorf("invalid group: %w", err)
					}
					dirCustomization.Group = group1
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
			return nil, HTTPErrorWithInternal(ErrorInvalidCustomization, err)
		}

		c.Directories = dirCustomizations
	}

	if rbpc.Files != nil {
		var fileCustomizations []blueprint.FileCustomization
		for _, f := range *rbpc.Files {
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
				user0, err := f.User.AsBlueprintFileUser0()
				if err == nil {
					fileCustomization.User = user0
				} else {
					user1, err := f.User.AsBlueprintFileUser1()
					if err != nil {
						return nil, fmt.Errorf("invalid user: %w", err)
					}
					fileCustomization.User = user1
				}
			}
			if f.Group != nil {
				group0, err := f.Group.AsBlueprintFileGroup0()
				if err == nil {
					fileCustomization.Group = group0
				} else {
					group1, err := f.Group.AsBlueprintFileGroup1()
					if err != nil {
						return nil, fmt.Errorf("invalid group: %w", err)
					}
					if group1 != 0 {
						fileCustomization.Group = group1
					}
				}
			}
			fileCustomizations = append(fileCustomizations, fileCustomization)
		}

		// Validate the file customizations, because the Cloud API does not use the custom unmarshaller
		_, err := blueprint.FileCustomizationsToFsNodeFiles(fileCustomizations)
		if err != nil {
			return nil, HTTPErrorWithInternal(ErrorInvalidCustomization, err)
		}

		c.Files = fileCustomizations
	}

	if rbpc.Repositories != nil {
		repoCustomizations := []blueprint.RepositoryCustomization{}
		for _, repo := range *rbpc.Repositories {
			repoCustomization := blueprint.RepositoryCustomization{
				Id: repo.Id,
			}

			if repo.Name != nil {
				repoCustomization.Name = *repo.Name
			}

			if repo.Filename != nil {
				repoCustomization.Filename = *repo.Filename
			}

			if repo.Baseurls != nil && len(*repo.Baseurls) > 0 {
				repoCustomization.BaseURLs = append(repoCustomization.BaseURLs, *repo.Baseurls...)
			}

			if repo.Gpgkeys != nil && len(*repo.Gpgkeys) > 0 {
				repoCustomization.GPGKeys = append(repoCustomization.GPGKeys, *repo.Gpgkeys...)
			}

			if repo.Gpgcheck != nil {
				repoCustomization.GPGCheck = repo.Gpgcheck
			}

			if repo.RepoGpgcheck != nil {
				repoCustomization.RepoGPGCheck = repo.RepoGpgcheck
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

			if repo.Sslverify != nil {
				repoCustomization.SSLVerify = repo.Sslverify
			}

			if repo.Priority != nil {
				repoCustomization.Priority = repo.Priority
			}

			if repo.ModuleHotfixes != nil {
				repoCustomization.ModuleHotfixes = repo.ModuleHotfixes
			}

			repoCustomizations = append(repoCustomizations, repoCustomization)
		}
		c.Repositories = repoCustomizations
	}

	if rbpc.Fips != nil {
		c.FIPS = rbpc.Fips
	}

	if installer := rbpc.Installer; installer != nil {
		c.Installer = &blueprint.InstallerCustomization{}
		if installer.Unattended != nil {
			c.Installer.Unattended = *installer.Unattended
		}
		if installer.SudoNopasswd != nil {
			c.Installer.SudoNopasswd = *installer.SudoNopasswd
		}
	}

	if rpm := rbpc.Rpm; rpm != nil && rpm.ImportKeys != nil {
		c.RPM = &blueprint.RPMCustomization{
			ImportKeys: &blueprint.RPMImportKeys{
				Files: *rpm.ImportKeys.Files,
			},
		}
	}

	if rhsm := rbpc.Rhsm; rhsm != nil && rhsm.Config != nil {
		bpRhsm := &blueprint.RHSMCustomization{
			Config: &blueprint.RHSMConfig{},
		}

		conf := rhsm.Config
		if conf.DnfPlugins != nil {
			bpRhsm.Config.DNFPlugins = &blueprint.SubManDNFPluginsConfig{}
			if conf.DnfPlugins.ProductId != nil && conf.DnfPlugins.ProductId.Enabled != nil {
				bpRhsm.Config.DNFPlugins.ProductID = &blueprint.DNFPluginConfig{
					Enabled: common.ToPtr(*conf.DnfPlugins.ProductId.Enabled),
				}
			}
			if conf.DnfPlugins.SubscriptionManager != nil && conf.DnfPlugins.SubscriptionManager.Enabled != nil {
				bpRhsm.Config.DNFPlugins.SubscriptionManager = &blueprint.DNFPluginConfig{
					Enabled: common.ToPtr(*conf.DnfPlugins.SubscriptionManager.Enabled),
				}
			}
		}

		if conf.SubscriptionManager != nil {
			bpRhsm.Config.SubscriptionManager = &blueprint.SubManConfig{}
			if conf.SubscriptionManager.Rhsm != nil && conf.SubscriptionManager.Rhsm.ManageRepos != nil {
				bpRhsm.Config.SubscriptionManager.RHSMConfig = &blueprint.SubManRHSMConfig{
					ManageRepos: common.ToPtr(*conf.SubscriptionManager.Rhsm.ManageRepos),
				}
			}
			if conf.SubscriptionManager.Rhsmcertd != nil && conf.SubscriptionManager.Rhsmcertd.AutoRegistration != nil {
				bpRhsm.Config.SubscriptionManager.RHSMCertdConfig = &blueprint.SubManRHSMCertdConfig{
					AutoRegistration: common.ToPtr(*conf.SubscriptionManager.Rhsmcertd.AutoRegistration),
				}
			}
		}

		c.RHSM = bpRhsm
	}

	disk, err := convertDiskCustomizations(rbpc.Disk)
	if err != nil {
		return nil, err
	}
	c.Disk = disk
	bpDisk, err := convertDiskCustomizations(rbpc.Disk)
	if err != nil {
		return nil, err
	}
	c.Disk = bpDisk

	return c, nil
}

// GetBlueprintFromCompose returns a base blueprint
// It is either constructed from the Blueprint passed in with the request, or it
// is an empty blueprint
func (request *ComposeRequest) GetBlueprintFromCompose() (blueprint.Blueprint, error) {
	// nil or blank blueprint returns a valid empty blueprint
	if request.Blueprint == nil || reflect.DeepEqual(*request.Blueprint, Blueprint{}) {
		bp := blueprint.Blueprint{Name: "empty blueprint"}
		err := bp.Initialize()
		return bp, err
	}

	return ConvertRequestBP(*request.Blueprint)
}

// ConvertRequestBP takes a request Blueprint and returns a composer blueprint.Blueprint
func ConvertRequestBP(rbp Blueprint) (blueprint.Blueprint, error) {
	var bp blueprint.Blueprint

	// Copy all the parts from the OpenAPI Blueprint into a blueprint.Blueprint
	// NOTE: Openapi fields may be nil, test for that first.
	bp.Name = rbp.Name
	if rbp.Description != nil {
		bp.Description = *rbp.Description
	}
	if rbp.Version != nil {
		bp.Version = *rbp.Version
	}
	if rbp.Distro != nil {
		bp.Distro = *rbp.Distro
	}

	if rbp.Packages != nil {
		for _, pkg := range *rbp.Packages {
			newPkg := blueprint.Package{Name: pkg.Name}
			if pkg.Version != nil {
				newPkg.Version = *pkg.Version
			}
			bp.Packages = append(bp.Packages, newPkg)
		}
	}

	if rbp.Modules != nil {
		for _, pkg := range *rbp.Modules {
			newPkg := blueprint.Package{Name: pkg.Name}
			if pkg.Version != nil {
				newPkg.Version = *pkg.Version
			}
			bp.Modules = append(bp.Modules, newPkg)
		}
	}

	if rbp.EnabledModules != nil {
		for _, em := range *rbp.EnabledModules {
			bp.EnabledModules = append(bp.EnabledModules, blueprint.EnabledModule{
				Name:   em.Name,
				Stream: em.Stream,
			})
		}
	}

	if rbp.Groups != nil {
		for _, group := range *rbp.Groups {
			bp.Groups = append(bp.Groups, blueprint.Group{
				Name: group.Name,
			})
		}
	}

	if rbp.Containers != nil {
		for _, c := range *rbp.Containers {
			newC := blueprint.Container{Source: c.Source, TLSVerify: c.TlsVerify}
			if c.Name != nil {
				newC.Name = *c.Name
			}
			bp.Containers = append(bp.Containers, newC)
		}
	}

	customizations, err := rbp.GetCustomizationsFromBlueprintRequest()
	if err != nil {
		return bp, err
	}
	bp.Customizations = customizations

	err = bp.Initialize()
	if err != nil {
		return bp, HTTPErrorWithInternal(ErrorFailedToInitializeBlueprint, err)
	}

	return bp, nil
}

// GetBlueprintFromCustomizations returns a new Blueprint with all of the
// customizations set from the ComposeRequest.Customizations
func (request *ComposeRequest) GetBlueprintFromCustomizations() (blueprint.Blueprint, error) {
	bp := blueprint.Blueprint{Name: "empty blueprint"}
	err := bp.Initialize()
	if err != nil {
		return bp, HTTPErrorWithInternal(ErrorFailedToInitializeBlueprint, err)
	}
	if request.Customizations == nil {
		return bp, nil
	}
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
					Name:     user.Name,
					Key:      user.Key,
					Password: user.Password,
					Groups:   groups,
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

	if request.Customizations.EnabledModules != nil {
		for _, em := range *request.Customizations.EnabledModules {
			bp.EnabledModules = append(bp.EnabledModules, blueprint.EnabledModule{
				Name:   em.Name,
				Stream: em.Stream,
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
				user0, err := d.User.AsDirectoryUser0()
				if err == nil {
					dirCustomization.User = user0
				} else {
					user1, err := d.User.AsDirectoryUser1()
					if err != nil {
						return bp, fmt.Errorf("invalid user: %w", err)
					}
					dirCustomization.User = user1
				}
			}
			if d.Group != nil {
				group0, err := d.Group.AsDirectoryGroup0()
				if err == nil {
					dirCustomization.Group = group0
				} else {
					group1, err := d.Group.AsDirectoryGroup1()
					if err != nil {
						return bp, fmt.Errorf("invalid group: %w", err)
					}
					dirCustomization.Group = group1
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
				user0, err := f.User.AsFileUser0()
				if err == nil {
					fileCustomization.User = user0
				} else {
					user1, err := f.User.AsFileUser1()
					if err != nil {
						return bp, fmt.Errorf("invalid user: %w", err)
					}
					fileCustomization.User = user1
				}
			}
			if f.Group != nil {
				group0, err := f.Group.AsFileGroup0()
				if err == nil {
					fileCustomization.Group = group0
				} else {
					group1, err := f.Group.AsFileGroup1()
					if err != nil {
						return bp, fmt.Errorf("invalid group: %w", err)
					}
					fileCustomization.Group = group1
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
		if request.Customizations.Services.Masked != nil {
			servicesCustomization.Masked = make([]string, len(*request.Customizations.Services.Masked))
			copy(servicesCustomization.Masked, *request.Customizations.Services.Masked)
		}
		bp.Customizations.Services = servicesCustomization
	}

	if request.Customizations.Openscap != nil {
		openSCAPCustomization := &blueprint.OpenSCAPCustomization{
			ProfileID: request.Customizations.Openscap.ProfileId,
		}

		if request.Customizations.Openscap.PolicyId != nil {
			openSCAPCustomization.PolicyID = request.Customizations.Openscap.PolicyId.String()
		}

		if request.Customizations.Openscap.Tailoring != nil && request.Customizations.Openscap.JsonTailoring != nil {
			return bp, fmt.Errorf("OpenSCAP customization error: choose one option between OpenSCAP tailoring and OpenSCAP json tailoring")
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
		if jsonTailoring := request.Customizations.Openscap.JsonTailoring; jsonTailoring != nil {
			openSCAPCustomization.JSONTailoring = &blueprint.OpenSCAPJSONTailoringCustomizations{
				ProfileID: jsonTailoring.ProfileId,
				Filepath:  jsonTailoring.Filepath,
			}
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

	if request.Customizations.Installer != nil {
		installer := &blueprint.InstallerCustomization{}
		if request.Customizations.Installer.Unattended != nil {
			installer.Unattended = *request.Customizations.Installer.Unattended
		}
		if request.Customizations.Installer.SudoNopasswd != nil {
			installer.SudoNopasswd = *request.Customizations.Installer.SudoNopasswd
		}
		bp.Customizations.Installer = installer
	}

	if request.Customizations.Rpm != nil && request.Customizations.Rpm.ImportKeys != nil {
		bp.Customizations.RPM = &blueprint.RPMCustomization{
			ImportKeys: &blueprint.RPMImportKeys{
				Files: *request.Customizations.Rpm.ImportKeys.Files,
			},
		}
	}

	if rhsm := request.Customizations.Rhsm; rhsm != nil && rhsm.Config != nil {
		bpRhsm := &blueprint.RHSMCustomization{
			Config: &blueprint.RHSMConfig{},
		}

		conf := rhsm.Config
		if conf.DnfPlugins != nil {
			bpRhsm.Config.DNFPlugins = &blueprint.SubManDNFPluginsConfig{}
			if conf.DnfPlugins.ProductId != nil && conf.DnfPlugins.ProductId.Enabled != nil {
				bpRhsm.Config.DNFPlugins.ProductID = &blueprint.DNFPluginConfig{
					Enabled: common.ToPtr(*conf.DnfPlugins.ProductId.Enabled),
				}
			}
			if conf.DnfPlugins.SubscriptionManager != nil && conf.DnfPlugins.SubscriptionManager.Enabled != nil {
				bpRhsm.Config.DNFPlugins.SubscriptionManager = &blueprint.DNFPluginConfig{
					Enabled: common.ToPtr(*conf.DnfPlugins.SubscriptionManager.Enabled),
				}
			}
		}

		if conf.SubscriptionManager != nil {
			bpRhsm.Config.SubscriptionManager = &blueprint.SubManConfig{}
			if conf.SubscriptionManager.Rhsm != nil && conf.SubscriptionManager.Rhsm.ManageRepos != nil {
				bpRhsm.Config.SubscriptionManager.RHSMConfig = &blueprint.SubManRHSMConfig{
					ManageRepos: common.ToPtr(*conf.SubscriptionManager.Rhsm.ManageRepos),
				}
			}
			if conf.SubscriptionManager.Rhsmcertd != nil && conf.SubscriptionManager.Rhsmcertd.AutoRegistration != nil {
				bpRhsm.Config.SubscriptionManager.RHSMCertdConfig = &blueprint.SubManRHSMCertdConfig{
					AutoRegistration: common.ToPtr(*conf.SubscriptionManager.Rhsmcertd.AutoRegistration),
				}
			}
		}

		bp.Customizations.RHSM = bpRhsm

	}

	bp.Customizations.Disk, err = convertDiskCustomizations(request.Customizations.Disk)
	if err != nil {
		return bp, err
	}

	if cacerts := request.Customizations.Cacerts; cacerts != nil {
		bp.Customizations.CACerts = &blueprint.CACustomization{
			PEMCerts: cacerts.PemCerts,
		}
	}

	// Did bp.Customizations get set at all? If not, set it back to nil
	if reflect.DeepEqual(*bp.Customizations, blueprint.Customizations{}) {
		bp.Customizations = nil
	}

	err = bp.CryptPasswords()
	if err != nil {
		return bp, fmt.Errorf("Error hashing passwords: %s", err.Error())
	}
	return bp, nil
}

// GetBlueprint returns a blueprint
// If the compose request includes a blueprint, return it, otherwise if it has
// customizations create a blueprint with those customizations. If it has neither
// return an empty blueprint.
func (request *ComposeRequest) GetBlueprint() (blueprint.Blueprint, error) {
	if request.Blueprint != nil {
		return request.GetBlueprintFromCompose()
	}

	return request.GetBlueprintFromCustomizations()
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

// GetImageRequests converts a composeRequest structure from the API to an intermediate imageRequest structure
// that's used for generating manifests and orchestrating worker jobs.
func (request *ComposeRequest) GetImageRequests(distroFactory *distrofactory.Factory, repoRegistry *reporegistry.RepoRegistry) ([]imageRequest, error) {
	// OpenAPI enforces blueprint or customization, not both
	// but check anyway
	if request.Customizations != nil && request.Blueprint != nil {
		return nil, HTTPError(ErrorBlueprintOrCustomNotBoth)
	}

	// Create a blueprint from the request
	bp, err := request.GetBlueprint()
	if err != nil {
		return nil, err
	}

	// Used when no repositories are included. Must be the original name because it may
	// be an alias and you cannot map back from the distrofactory to the original string.
	originalDistroName := request.Distribution

	// If there is a distribution in the blueprint it overrides the request's distro
	if len(bp.Distro) > 0 {
		originalDistroName = bp.Distro
	}
	distribution := distroFactory.GetDistro(originalDistroName)
	if distribution == nil {
		return nil, HTTPError(ErrorUnsupportedDistribution)
	}

	// add the user-defined repositories only to the depsolve job for the
	// payload (the packages for the final image)
	payloadRepositories := request.GetPayloadRepositories()

	// use the same seed for all images so we get the same IDs
	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, HTTPError(ErrorFailedToGenerateManifestSeed)
	}
	manifestSeed := bigSeed.Int64()

	// For backwards compatibility, we support both a single image request
	// as well as an array of requests in the API. Exactly one must be
	// specified.
	if request.ImageRequest != nil {
		if request.ImageRequests != nil {
			// we should really be using oneOf in the spec
			return nil, HTTPError(ErrorInvalidNumberOfImageBuilds)
		}
		request.ImageRequests = &[]ImageRequest{*request.ImageRequest}
	}
	if request.ImageRequests == nil {
		return nil, HTTPError(ErrorInvalidNumberOfImageBuilds)
	}
	var irs []imageRequest
	for _, ir := range *request.ImageRequests {
		arch, err := distribution.GetArch(ir.Architecture)
		if err != nil {
			return nil, HTTPError(ErrorUnsupportedArchitecture)
		}
		imageType, err := arch.GetImageType(imageTypeFromApiImageType(ir.ImageType, arch))
		if err != nil {
			return nil, HTTPError(ErrorUnsupportedImageType)
		}

		repos, err := convertRepos(ir.Repositories, payloadRepositories, imageType.PayloadPackageSets())
		if err != nil {
			return nil, err
		}

		// If no repositories are included with the imageRequest use the defaults for
		// the distro selected by the blueprint, or the compose request.
		if len(ir.Repositories) == 0 {
			dr, err := repoRegistry.ReposByImageTypeName(originalDistroName, arch.Name(), imageType.Name())
			if err != nil {
				return nil, err
			}
			repos = append(repos, dr...)
		}

		// Get the initial ImageOptions with image size set
		imageOptions := ir.GetImageOptions(imageType, bp)

		if request.Koji == nil {
			imageOptions.Facts = &facts.ImageOptions{
				APIType: facts.CLOUDV2_APITYPE,
			}
			oscap := bp.Customizations.GetOpenSCAP()
			if oscap != nil {
				if oscap.ProfileID != "" {
					imageOptions.Facts.OpenSCAPProfileID = oscap.ProfileID
				}
				if oscap.PolicyID != "" {
					policyID, err := uuid.Parse(oscap.PolicyID)
					if err != nil {
						return nil, fmt.Errorf("Unable to parse %s as a uuid: %w", oscap.PolicyID, err)
					}
					imageOptions.Facts.CompliancePolicyID = policyID
				}
			}
		}

		// Set Subscription from the compose request
		imageOptions.Subscription = request.GetSubscription()

		// Set PartitioningMode from the compose request
		imageOptions.PartitioningMode, err = request.GetPartitioningMode()
		if err != nil {
			return nil, err
		}

		// Set OSTree options from the image request
		imageOptions.OSTree, err = ir.GetOSTreeOptions()
		if err != nil {
			return nil, err
		}

		var irTargets []*target.Target
		if ir.UploadOptions == nil && (ir.UploadTargets == nil || len(*ir.UploadTargets) == 0) {
			// nowhere to put the image, this is a user error
			if request.Koji == nil {
				return nil, HTTPError(ErrorJSONUnMarshallingError)
			}
		} else {
			// Get the target for the selected image type
			irTargets, err = ir.GetTargets(request, imageType)
			if err != nil {
				return nil, err
			}
		}

		irs = append(irs, imageRequest{
			imageType:    imageType,
			repositories: repos,
			imageOptions: imageOptions,
			targets:      irTargets,
			blueprint:    bp,
			manifestSeed: manifestSeed,
		})
	}
	return irs, nil
}

func convertDiskCustomizations(disk *Disk) (*blueprint.DiskCustomization, error) {
	if disk == nil {
		return nil, nil
	}

	diskSize, err := decodeMinsize(disk.Minsize)
	if err != nil {
		return nil, err
	}
	bpDisk := &blueprint.DiskCustomization{
		MinSize: diskSize,
		Type:    string(common.DerefOrDefault(disk.Type)),
	}

	for idx, partition := range disk.Partitions {
		// partition successfully converts to all three types, so convert to
		// filesystem to sniff the type string
		sniffer, err := partition.AsFilesystemTyped()
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize disk customization partition %d", idx)
		}

		var bpPartition blueprint.PartitionCustomization
		switch partType := common.DerefOrDefault(sniffer.Type); string(partType) {
		case string(Plain):
			fs, err := partition.AsFilesystemTyped()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize disk customization partition %d with type %q", idx, partType)
			}
			fsSize, err := decodeMinsize(fs.Minsize)
			if err != nil {
				return nil, err
			}
			bpPartition = blueprint.PartitionCustomization{
				Type:     string(common.DerefOrDefault(fs.Type)),
				PartType: common.DerefOrDefault(fs.PartType),
				MinSize:  fsSize,
				FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
					Mountpoint: fs.Mountpoint,
					Label:      common.DerefOrDefault(fs.Label),
					FSType:     string(common.DerefOrDefault(fs.FsType)),
				},
			}
		case string(Btrfs):
			btrfsVol, err := partition.AsBtrfsVolume()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize disk customization partition %d with type %q", idx, partType)
			}
			volSize, err := decodeMinsize(btrfsVol.Minsize)
			if err != nil {
				return nil, err
			}

			bpPartition = blueprint.PartitionCustomization{
				Type:     string(common.DerefOrDefault(btrfsVol.Type)),
				PartType: common.DerefOrDefault(btrfsVol.PartType),
				MinSize:  volSize,
			}

			for _, subvol := range btrfsVol.Subvolumes {
				bpSubvol := blueprint.BtrfsSubvolumeCustomization{
					Name:       subvol.Name,
					Mountpoint: subvol.Mountpoint,
				}
				bpPartition.Subvolumes = append(bpPartition.Subvolumes, bpSubvol)
			}
		case string(Lvm):
			vg, err := partition.AsVolumeGroup()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize disk customization partition %d with type %q", idx, partType)
			}
			vgSize, err := decodeMinsize(vg.Minsize)
			if err != nil {
				return nil, err
			}
			bpPartition = blueprint.PartitionCustomization{
				Type:     string(common.DerefOrDefault(vg.Type)),
				PartType: common.DerefOrDefault(vg.PartType),
				MinSize:  vgSize,
				VGCustomization: blueprint.VGCustomization{
					Name: common.DerefOrDefault(vg.Name),
				},
			}

			for _, lv := range vg.LogicalVolumes {
				lvSize, err := decodeMinsize(lv.Minsize)
				if err != nil {
					return nil, err
				}
				bpLV := blueprint.LVCustomization{
					Name:    common.DerefOrDefault(lv.Name),
					MinSize: lvSize,
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						Mountpoint: lv.Mountpoint,
						Label:      common.DerefOrDefault(lv.Label),
						FSType:     string(common.DerefOrDefault(lv.FsType)),
					},
				}
				bpPartition.LogicalVolumes = append(bpPartition.LogicalVolumes, bpLV)
			}
		default:
			return nil, fmt.Errorf("disk customization partition %d has invalid or unknown type %q", idx, partType)

		}
		bpDisk.Partitions = append(bpDisk.Partitions, bpPartition)
	}

	return bpDisk, nil
}

func decodeMinsize(size *Minsize) (uint64, error) {
	if size == nil {
		return 0, nil
	}

	if sizeStr, err := size.AsMinsize1(); err == nil {
		return datasizes.Parse(sizeStr)
	}

	if sizeUint, err := size.AsMinsize0(); err == nil {
		return sizeUint, err
	}

	return 0, fmt.Errorf("failed to convert value \"%v\" to number", size)
}
