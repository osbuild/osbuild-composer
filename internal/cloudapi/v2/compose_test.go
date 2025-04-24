package v2

import (
	"fmt"
	"io/fs"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	repos "github.com/osbuild/images/data/repositories"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Change these when moving to a new release
const (
	TEST_DISTRO_NAME    = "fedora-42"
	TEST_DISTRO_VERSION = "42"
)

// GetTestBlueprint returns a populated blueprint
// This is used in testing the Customizations compose request
// and the Blueprint compose request.
// They both result in the same final blueprint used to create the compose
func GetTestBlueprint() blueprint.Blueprint {
	// Test with customizations
	expected := blueprint.Blueprint{Name: "empty blueprint"}
	err := expected.Initialize()
	// An empty blueprint should never fail to initialize
	if err != nil {
		panic(err)
	}

	// Construct the expected blueprint result
	// Packages
	expected.Packages = []blueprint.Package{
		{Name: "bash"},
		{Name: "tmux"},
	}

	expected.EnabledModules = []blueprint.EnabledModule{
		{
			Name:   "node",
			Stream: "20",
		},
	}

	// Containers
	expected.Containers = []blueprint.Container{
		blueprint.Container{
			Name:   "container-name",
			Source: "http://some.path.to/a/container/source",
		},
	}

	// Customizations
	expected.Customizations = &blueprint.Customizations{
		User: []blueprint.UserCustomization{
			blueprint.UserCustomization{
				Name:     "admin",
				Key:      common.ToPtr("dummy ssh-key"),
				Password: common.ToPtr("$6$secret-password"),
				Groups:   []string{"users", "wheel"},
			},
		},
		Directories: []blueprint.DirectoryCustomization{
			blueprint.DirectoryCustomization{
				Path:          "/opt/extra",
				User:          "root",
				Group:         "root",
				Mode:          "0755",
				EnsureParents: true,
			},
		},
		Files: []blueprint.FileCustomization{
			blueprint.FileCustomization{
				Path:  "/etc/mad.conf",
				User:  "root",
				Group: "root",
				Mode:  "0644",
				Data:  "Alfred E. Neuman was here.\n",
			},
		},
		Filesystem: []blueprint.FilesystemCustomization{
			blueprint.FilesystemCustomization{
				Mountpoint: "/var/lib/wopr/",
				MinSize:    1099511627776,
			},
		},
		Services: &blueprint.ServicesCustomization{
			Enabled:  []string{"sshd"},
			Disabled: []string{"cleanup"},
			Masked:   []string{"firewalld"},
		},
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID: "B 263-59",
		},
		Repositories: []blueprint.RepositoryCustomization{
			blueprint.RepositoryCustomization{
				Id:             "custom repo",
				Metalink:       "http://example.org/metalink",
				Enabled:        common.ToPtr(true),
				GPGCheck:       common.ToPtr(true),
				ModuleHotfixes: common.ToPtr(true),
			},
		},
		Firewall: &blueprint.FirewallCustomization{
			Ports: []string{
				"22/tcp",
			},
		},
		Hostname:           common.ToPtr("hostname"),
		InstallationDevice: "/dev/sda",
		Kernel: &blueprint.KernelCustomization{
			Append: "nosmt=force",
			Name:   "kernel-debug",
		},
		Locale: &blueprint.LocaleCustomization{
			Keyboard: common.ToPtr("us"),
			Languages: []string{
				"en_US.UTF-8",
			},
		},
		FDO: &blueprint.FDOCustomization{
			DiunPubKeyHash:          "pubkeyhash",
			DiunPubKeyInsecure:      "pubkeyinsecure",
			DiunPubKeyRootCerts:     "pubkeyrootcerts",
			ManufacturingServerURL:  "serverurl",
			DiMfgStringTypeMacIface: "iface",
		},
		Ignition: &blueprint.IgnitionCustomization{
			FirstBoot: &blueprint.FirstBootIgnitionCustomization{
				ProvisioningURL: "provisioning-url.local",
			},
		},
		Timezone: &blueprint.TimezoneCustomization{
			Timezone: common.ToPtr("US/Eastern"),
			NTPServers: []string{
				"0.north-america.pool.ntp.org",
				"1.north-america.pool.ntp.org",
			},
		},
		FIPS: common.ToPtr(true),
		Installer: &blueprint.InstallerCustomization{
			Unattended:   true,
			SudoNopasswd: []string{`%wheel`},
		},
		RPM: &blueprint.RPMCustomization{
			ImportKeys: &blueprint.RPMImportKeys{
				Files: []string{
					"/root/gpg-key",
				},
			},
		},
		RHSM: &blueprint.RHSMCustomization{
			Config: &blueprint.RHSMConfig{
				DNFPlugins: &blueprint.SubManDNFPluginsConfig{
					ProductID: &blueprint.DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: &blueprint.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubscriptionManager: &blueprint.SubManConfig{
					RHSMConfig: &blueprint.SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					RHSMCertdConfig: &blueprint.SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		Disk: &blueprint.DiskCustomization{
			MinSize: 20 * datasizes.GiB,
			Partitions: []blueprint.PartitionCustomization{
				{
					Type: "plain",
					FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
						FSType:     "xfs",
						Label:      "data",
						Mountpoint: "/data",
					},
				},
				{
					Type:    "btrfs",
					MinSize: 100 * datasizes.MiB,
					BtrfsVolumeCustomization: blueprint.BtrfsVolumeCustomization{
						Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
							{
								Name:       "+subvols/db1",
								Mountpoint: "/data/db1",
							},
							{
								Name:       "+subvols/db2",
								Mountpoint: "/data/db2",
							},
						},
					},
				},
				{
					Type:     "lvm",
					MinSize:  10 * datasizes.GiB,
					PartType: "E6D6D379-F507-44C2-A23C-238F2A3DF928",
					VGCustomization: blueprint.VGCustomization{
						Name: "vg000001",
						LogicalVolumes: []blueprint.LVCustomization{
							{
								Name: "rootlv",
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/",
									FSType:     "ext4",
								},
							},
							{
								Name:    "homelv",
								MinSize: 3 * datasizes.GiB,
								FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
									Mountpoint: "/home",
									Label:      "home",
								},
							},
						},
					},
				},
			},
		},
	}

	return expected
}

func TestGetBlueprintFromCustomizations(t *testing.T) {
	// Empty request should return empty blueprint
	cr := ComposeRequest{}
	bp, err := cr.GetBlueprintFromCustomizations()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	// Empty request should return empty blueprint
	cr = ComposeRequest{
		Customizations: &Customizations{},
	}
	bp, err = cr.GetBlueprintFromCustomizations()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	var dirUser Directory_User
	require.NoError(t, dirUser.FromDirectoryUser0("root"))
	var dirGroup Directory_Group
	require.NoError(t, dirGroup.FromDirectoryGroup0("root"))
	var fileUser File_User
	require.NoError(t, fileUser.FromFileUser0("root"))
	var fileGroup File_Group
	require.NoError(t, fileGroup.FromFileGroup0("root"))

	var plainPart Partition
	require.NoError(t, plainPart.FromFilesystemTyped(
		FilesystemTyped{
			Type:       common.ToPtr(Plain),
			Minsize:    nil,
			FsType:     common.ToPtr(FilesystemTypedFsTypeXfs),
			Label:      common.ToPtr("data"),
			Mountpoint: "/data",
		},
	))

	var btrfsSize Minsize
	require.NoError(t, btrfsSize.FromMinsize0(100*datasizes.MiB))

	var btrfsPart Partition
	require.NoError(t, btrfsPart.FromBtrfsVolume(
		BtrfsVolume{
			Type:    common.ToPtr(Btrfs),
			Minsize: &btrfsSize,
			Subvolumes: []BtrfsSubvolume{
				{
					Mountpoint: "/data/db1",
					Name:       "+subvols/db1",
				},
				{
					Mountpoint: "/data/db2",
					Name:       "+subvols/db2",
				},
			},
		},
	))

	var vgSize, lvSize Minsize
	require.NoError(t, vgSize.FromMinsize0(10*datasizes.GiB))
	require.NoError(t, lvSize.FromMinsize0(3*datasizes.GiB))

	var vgPart Partition
	require.NoError(t, vgPart.FromVolumeGroup(
		VolumeGroup{
			Type:     common.ToPtr(Lvm),
			Name:     common.ToPtr("vg000001"),
			Minsize:  &vgSize,
			PartType: common.ToPtr("E6D6D379-F507-44C2-A23C-238F2A3DF928"),
			LogicalVolumes: []LogicalVolume{
				{
					FsType:     common.ToPtr(LogicalVolumeFsTypeExt4),
					Label:      nil,
					Minsize:    nil,
					Mountpoint: "/",
					Name:       common.ToPtr("rootlv"),
				},
				{
					FsType:     nil,
					Label:      common.ToPtr("home"),
					Minsize:    &lvSize,
					Mountpoint: "/home",
					Name:       common.ToPtr("homelv"),
				},
			},
		},
	))

	var diskSize Minsize
	require.NoError(t, diskSize.FromMinsize1("20 GiB"))

	// Construct the compose request with customizations
	cr = ComposeRequest{Customizations: &Customizations{
		Users: &[]User{
			User{
				Name:     "admin",
				Key:      common.ToPtr("dummy ssh-key"),
				Password: common.ToPtr("$6$secret-password"),
				Groups:   &[]string{"users", "wheel"},
			}},
		Packages: &[]string{"bash", "tmux"},
		EnabledModules: &[]Module{
			{
				Name:   "node",
				Stream: "20",
			},
		},
		Containers: &[]Container{
			Container{
				Name:   common.ToPtr("container-name"),
				Source: "http://some.path.to/a/container/source",
			},
		},
		Directories: &[]Directory{
			Directory{
				Path:          "/opt/extra",
				EnsureParents: common.ToPtr(true),
				User:          &dirUser,
				Group:         &dirGroup,
				Mode:          common.ToPtr("0755"),
			},
		},
		Files: &[]File{
			File{
				Path:          "/etc/mad.conf",
				Data:          common.ToPtr("Alfred E. Neuman was here.\n"),
				EnsureParents: common.ToPtr(true),
				User:          &fileUser,
				Group:         &fileGroup,
				Mode:          common.ToPtr("0644"),
			},
		},
		Filesystem: &[]Filesystem{
			Filesystem{
				Mountpoint: "/var/lib/wopr/",
				MinSize:    1099511627776,
			},
		},
		Services: &Services{
			Disabled: &[]string{"cleanup"},
			Enabled:  &[]string{"sshd"},
			Masked:   &[]string{"firewalld"},
		},
		Openscap: &OpenSCAP{ProfileId: "B 263-59"},
		CustomRepositories: &[]CustomRepository{
			CustomRepository{
				Id:             "custom repo",
				Metalink:       common.ToPtr("http://example.org/metalink"),
				CheckGpg:       common.ToPtr(true),
				Enabled:        common.ToPtr(true),
				ModuleHotfixes: common.ToPtr(true),
			},
		},
		Firewall: &FirewallCustomization{
			Ports: common.ToPtr([]string{
				"22/tcp",
			}),
		},
		Hostname:           common.ToPtr("hostname"),
		InstallationDevice: common.ToPtr("/dev/sda"),
		Kernel: &Kernel{
			Append: common.ToPtr("nosmt=force"),
			Name:   common.ToPtr("kernel-debug"),
		},
		Locale: &Locale{
			Keyboard: common.ToPtr("us"),
			Languages: common.ToPtr([]string{
				"en_US.UTF-8",
			}),
		},
		Fdo: &FDO{
			DiunPubKeyHash:          common.ToPtr("pubkeyhash"),
			DiunPubKeyInsecure:      common.ToPtr("pubkeyinsecure"),
			DiunPubKeyRootCerts:     common.ToPtr("pubkeyrootcerts"),
			DiMfgStringTypeMacIface: common.ToPtr("iface"),
			ManufacturingServerUrl:  common.ToPtr("serverurl"),
		},
		Ignition: &Ignition{
			Firstboot: &IgnitionFirstboot{
				Url: "provisioning-url.local",
			},
		},
		Timezone: &Timezone{
			Timezone: common.ToPtr("US/Eastern"),
			Ntpservers: common.ToPtr([]string{
				"0.north-america.pool.ntp.org",
				"1.north-america.pool.ntp.org",
			}),
		},
		Fips: &FIPS{Enabled: common.ToPtr(true)},
		Installer: &Installer{
			Unattended:   common.ToPtr(true),
			SudoNopasswd: &[]string{`%wheel`},
		},
		Rpm: &RPMCustomization{
			ImportKeys: &ImportKeys{
				Files: &[]string{
					"/root/gpg-key",
				},
			},
		},
		Rhsm: &RHSMCustomization{
			Config: &RHSMConfig{
				DnfPlugins: &SubManDNFPluginsConfig{
					ProductId: &DNFPluginConfig{
						Enabled: common.ToPtr(true),
					},
					SubscriptionManager: &DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
				SubscriptionManager: &SubManConfig{
					Rhsm: &SubManRHSMConfig{
						ManageRepos: common.ToPtr(true),
					},
					Rhsmcertd: &SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(false),
					},
				},
			},
		},
		Disk: &Disk{
			Minsize: &diskSize,
			Partitions: []Partition{
				plainPart,
				btrfsPart,
				vgPart,
			},
		},
	}}

	bp, err = cr.GetBlueprintFromCustomizations()
	require.Nil(t, err)
	assert.Equal(t, GetTestBlueprint(), bp)
}

func TestBlueprintFromCustomizationPasswordsHashed(t *testing.T) {
	// Construct the compose request with customizations
	plaintextPassword := "secret-password"
	cr := ComposeRequest{Customizations: &Customizations{
		Users: &[]User{
			User{
				Name:     "admin",
				Key:      common.ToPtr("dummy ssh-key"),
				Password: common.ToPtr(plaintextPassword),
				Groups:   &[]string{"users", "wheel"},
			}},
	}}

	bp, err := cr.GetBlueprintFromCustomizations()
	require.Nil(t, err)
	// Passwords should be hashed
	assert.NotEqual(t, plaintextPassword, bp.Customizations.User[0].Password)
}

func TestGetBlueprintFromCompose(t *testing.T) {
	// Empty request should return empty blueprint
	cr := ComposeRequest{}
	bp, err := cr.GetBlueprintFromCompose()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	// Empty request should return empty blueprint
	cr = ComposeRequest{
		Blueprint: &Blueprint{},
	}
	bp, err = cr.GetBlueprintFromCompose()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	var dirUser Directory_User
	require.NoError(t, dirUser.FromDirectoryUser0("root"))
	var dirGroup Directory_Group
	require.NoError(t, dirGroup.FromDirectoryGroup0("root"))
	var fileUser BlueprintFile_User
	require.NoError(t, fileUser.FromBlueprintFileUser0("root"))
	var fileGroup BlueprintFile_Group
	require.NoError(t, fileGroup.FromBlueprintFileGroup0("root"))

	var plainPart Partition
	require.NoError(t, plainPart.FromFilesystemTyped(
		FilesystemTyped{
			Type:       common.ToPtr(Plain),
			Minsize:    nil,
			FsType:     common.ToPtr(FilesystemTypedFsTypeXfs),
			Label:      common.ToPtr("data"),
			Mountpoint: "/data",
		},
	))

	var btrfsSize Minsize
	require.NoError(t, btrfsSize.FromMinsize0(100*datasizes.MiB))

	var btrfsPart Partition
	require.NoError(t, btrfsPart.FromBtrfsVolume(
		BtrfsVolume{
			Type:    common.ToPtr(Btrfs),
			Minsize: &btrfsSize,
			Subvolumes: []BtrfsSubvolume{
				{
					Mountpoint: "/data/db1",
					Name:       "+subvols/db1",
				},
				{
					Mountpoint: "/data/db2",
					Name:       "+subvols/db2",
				},
			},
		},
	))

	var vgSize, lvSize Minsize
	require.NoError(t, vgSize.FromMinsize0(10*datasizes.GiB))
	require.NoError(t, lvSize.FromMinsize0(3*datasizes.GiB))

	var vgPart Partition
	require.NoError(t, vgPart.FromVolumeGroup(
		VolumeGroup{
			Type:     common.ToPtr(Lvm),
			Minsize:  &vgSize,
			Name:     common.ToPtr("vg000001"),
			PartType: common.ToPtr("E6D6D379-F507-44C2-A23C-238F2A3DF928"),
			LogicalVolumes: []LogicalVolume{
				{
					FsType:     common.ToPtr(LogicalVolumeFsTypeExt4),
					Label:      nil,
					Minsize:    nil,
					Mountpoint: "/",
					Name:       common.ToPtr("rootlv"),
				},
				{
					FsType:     nil,
					Label:      common.ToPtr("home"),
					Minsize:    &lvSize,
					Mountpoint: "/home",
					Name:       common.ToPtr("homelv"),
				},
			},
		},
	))

	var fsSize Minsize
	require.NoError(t, fsSize.FromMinsize0(1099511627776))

	var diskSize Minsize
	require.NoError(t, diskSize.FromMinsize1("20 GiB"))

	// Construct the compose request with a blueprint
	cr = ComposeRequest{Blueprint: &Blueprint{
		Name:     "empty blueprint",
		Version:  common.ToPtr("0.0.0"),
		Packages: &[]Package{{Name: "bash"}, {Name: "tmux"}},
		EnabledModules: &[]Module{
			{
				Name:   "node",
				Stream: "20",
			},
		},
		Containers: &[]Container{
			Container{
				Name:   common.ToPtr("container-name"),
				Source: "http://some.path.to/a/container/source",
			},
		},
		Customizations: &BlueprintCustomizations{
			User: &[]BlueprintUser{
				{
					Name:     "admin",
					Key:      common.ToPtr("dummy ssh-key"),
					Password: common.ToPtr("$6$secret-password"),
					Groups:   &[]string{"users", "wheel"},
				}},
			Directories: &[]Directory{
				Directory{
					Path:          "/opt/extra",
					EnsureParents: common.ToPtr(true),
					User:          &dirUser,
					Group:         &dirGroup,
					Mode:          common.ToPtr("0755"),
				},
			},
			Files: &[]BlueprintFile{
				{
					Path:  "/etc/mad.conf",
					Data:  common.ToPtr("Alfred E. Neuman was here.\n"),
					User:  &fileUser,
					Group: &fileGroup,
					Mode:  common.ToPtr("0644"),
				},
			},
			Filesystem: &[]BlueprintFilesystem{
				{
					Mountpoint: "/var/lib/wopr/",
					Minsize:    fsSize,
				},
			},
			Services: &Services{
				Disabled: &[]string{"cleanup"},
				Enabled:  &[]string{"sshd"},
				Masked:   &[]string{"firewalld"},
			},
			Openscap: &BlueprintOpenSCAP{ProfileId: "B 263-59"},
			Repositories: &[]BlueprintRepository{
				{
					Id:             "custom repo",
					Metalink:       common.ToPtr("http://example.org/metalink"),
					Gpgcheck:       common.ToPtr(true),
					Enabled:        common.ToPtr(true),
					ModuleHotfixes: common.ToPtr(true),
				},
			},
			Firewall: &BlueprintFirewall{
				Ports: common.ToPtr([]string{
					"22/tcp",
				}),
			},
			Hostname:           common.ToPtr("hostname"),
			InstallationDevice: common.ToPtr("/dev/sda"),
			Kernel: &Kernel{
				Append: common.ToPtr("nosmt=force"),
				Name:   common.ToPtr("kernel-debug"),
			},
			Locale: &Locale{
				Keyboard: common.ToPtr("us"),
				Languages: common.ToPtr([]string{
					"en_US.UTF-8",
				}),
			},
			Fdo: &FDO{
				DiunPubKeyHash:          common.ToPtr("pubkeyhash"),
				DiunPubKeyInsecure:      common.ToPtr("pubkeyinsecure"),
				DiunPubKeyRootCerts:     common.ToPtr("pubkeyrootcerts"),
				DiMfgStringTypeMacIface: common.ToPtr("iface"),
				ManufacturingServerUrl:  common.ToPtr("serverurl"),
			},
			Ignition: &Ignition{
				Firstboot: &IgnitionFirstboot{
					Url: "provisioning-url.local",
				},
			},
			Timezone: &Timezone{
				Timezone: common.ToPtr("US/Eastern"),
				Ntpservers: common.ToPtr([]string{
					"0.north-america.pool.ntp.org",
					"1.north-america.pool.ntp.org",
				}),
			},
			Fips: common.ToPtr(true),
			Installer: &Installer{
				Unattended:   common.ToPtr(true),
				SudoNopasswd: &[]string{`%wheel`},
			},
			Rpm: &RPMCustomization{
				ImportKeys: &ImportKeys{
					Files: &[]string{
						"/root/gpg-key",
					},
				},
			},
			Rhsm: &RHSMCustomization{
				Config: &RHSMConfig{
					DnfPlugins: &SubManDNFPluginsConfig{
						ProductId: &DNFPluginConfig{
							Enabled: common.ToPtr(true),
						},
						SubscriptionManager: &DNFPluginConfig{
							Enabled: common.ToPtr(false),
						},
					},
					SubscriptionManager: &SubManConfig{
						Rhsm: &SubManRHSMConfig{
							ManageRepos: common.ToPtr(true),
						},
						Rhsmcertd: &SubManRHSMCertdConfig{
							AutoRegistration: common.ToPtr(false),
						},
					},
				},
			},
			Disk: &Disk{
				Minsize: &diskSize,
				Partitions: []Partition{
					plainPart,
					btrfsPart,
					vgPart,
				},
			},
		},
	}}

	bp, err = cr.GetBlueprintFromCompose()
	require.Nil(t, err)
	assert.Equal(t, GetTestBlueprint(), bp)
}

func TestGetBlueprint(t *testing.T) {
	cr := ComposeRequest{}
	bp, err := cr.GetBlueprint()
	require.Nil(t, err)
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)
}

func TestGetPayloadRepositories(t *testing.T) {

	// Empty PayloadRepositories
	cr := ComposeRequest{}
	repos := cr.GetPayloadRepositories()
	assert.Len(t, repos, 0)
	assert.Equal(t, []Repository(nil), repos)

	// Populated PayloadRepositories
	cr = ComposeRequest{Customizations: &Customizations{
		PayloadRepositories: &[]Repository{
			Repository{
				Baseurl:        common.ToPtr("http://example.org/pub/linux/repo"),
				CheckGpg:       common.ToPtr(true),
				PackageSets:    &[]string{"build", "archive"},
				Rhsm:           common.ToPtr(false),
				ModuleHotfixes: common.ToPtr(true),
			},
		},
	}}

	expected := []Repository{
		Repository{
			Baseurl:        common.ToPtr("http://example.org/pub/linux/repo"),
			CheckGpg:       common.ToPtr(true),
			PackageSets:    &[]string{"build", "archive"},
			Rhsm:           common.ToPtr(false),
			ModuleHotfixes: common.ToPtr(true),
		},
	}
	repos = cr.GetPayloadRepositories()
	assert.Len(t, repos, 1)
	assert.Equal(t, expected, repos)
}

func TestGetSubscriptions(t *testing.T) {
	// Empty Subscription
	cr := ComposeRequest{}
	sub := cr.GetSubscription()
	assert.Equal(t, (*subscription.ImageOptions)(nil), sub)

	// Populated Subscription
	cr = ComposeRequest{Customizations: &Customizations{
		Subscription: common.ToPtr(Subscription{
			ActivationKey: "key",
			BaseUrl:       "http://example.org/baseurl",
			Insights:      false,
			Organization:  "Weyland-Yutani",
			ServerUrl:     "http://example.org/serverurl",
		}),
	}}

	expected := &subscription.ImageOptions{
		ActivationKey: "key",
		BaseUrl:       "http://example.org/baseurl",
		Insights:      false,
		Organization:  "Weyland-Yutani",
		ServerUrl:     "http://example.org/serverurl",
	}
	sub = cr.GetSubscription()
	assert.Equal(t, expected, sub)

}

func TestGetPartitioningMode(t *testing.T) {
	// Empty Partitioning Mode
	cr := ComposeRequest{}
	_, err := cr.GetPartitioningMode()
	assert.NoError(t, err)

	// Populated PartitioningMode
	cr = ComposeRequest{Customizations: &Customizations{
		PartitioningMode: common.ToPtr(CustomizationsPartitioningModeAutoLvm),
	}}
	pm, err := cr.GetPartitioningMode()
	assert.NoError(t, err)
	assert.Equal(t, disk.AutoLVMPartitioningMode, pm)
}

func TestGetImageRequests_ImageTypeConversion(t *testing.T) {
	// The focus of this test is to ensure that the image type enumeration in the public Cloud API is correctly
	// translated to the image type names as defined in the images library.
	// Additionally, it covers that the default target is correctly set.

	fedora := "fedora-41"
	rhel8 := "rhel-8.10"
	centos8 := "centos-8"
	rhel9 := "rhel-9.10"
	centos9 := "centos-9"
	tests := []struct {
		requestedImageType ImageTypes
		requestedDistros   []string
		expectedImageType  string
		expectedTargetName target.TargetName
	}{
		{
			requestedImageType: ImageTypesAws,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "ami",
			expectedTargetName: target.TargetNameAWS,
		},
		{
			requestedImageType: ImageTypesAws,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-ami",
			expectedTargetName: target.TargetNameAWS,
		},
		{
			requestedImageType: ImageTypesAwsHaRhui,
			requestedDistros:   []string{rhel8, rhel9},
			expectedImageType:  "ec2-ha",
			expectedTargetName: target.TargetNameAWS,
		},
		{
			requestedImageType: ImageTypesAwsRhui,
			requestedDistros:   []string{rhel8, rhel9},
			expectedImageType:  "ec2",
			expectedTargetName: target.TargetNameAWS,
		},
		{
			requestedImageType: ImageTypesAwsSapRhui,
			requestedDistros:   []string{rhel8, rhel9},
			expectedImageType:  "ec2-sap",
			expectedTargetName: target.TargetNameAWS,
		},
		{
			requestedImageType: ImageTypesAzure,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "vhd",
			expectedTargetName: target.TargetNameAzureImage,
		},
		{
			requestedImageType: ImageTypesAzure,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-vhd",
			expectedTargetName: target.TargetNameAzureImage,
		},
		{
			requestedImageType: ImageTypesAzureEap7Rhui,
			requestedDistros:   []string{rhel8},
			expectedImageType:  "azure-eap7-rhui",
			expectedTargetName: target.TargetNameAzureImage,
		},
		{
			requestedImageType: ImageTypesAzureRhui,
			requestedDistros:   []string{rhel8, rhel9},
			expectedImageType:  "azure-rhui",
			expectedTargetName: target.TargetNameAzureImage,
		},
		{
			requestedImageType: ImageTypesAzureSapRhui,
			requestedDistros:   []string{rhel8, rhel9},
			expectedImageType:  "azure-sap-rhui",
			expectedTargetName: target.TargetNameAzureImage,
		},
		{
			requestedImageType: ImageTypesEdgeCommit,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "edge-commit",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesEdgeContainer,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "edge-container",
			expectedTargetName: target.TargetNameContainer,
		},
		{
			requestedImageType: ImageTypesEdgeInstaller,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "edge-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesGcp,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "gce",
			expectedTargetName: target.TargetNameGCP,
		},
		{
			requestedImageType: ImageTypesGcpRhui,
			requestedDistros:   []string{rhel9},
			expectedImageType:  "gce",
			expectedTargetName: target.TargetNameGCP,
		},
		{
			requestedImageType: ImageTypesGcpRhui,
			requestedDistros:   []string{rhel8},
			expectedImageType:  "gce-rhui",
			expectedTargetName: target.TargetNameGCP,
		},
		{
			requestedImageType: ImageTypesGuestImage,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "qcow2",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesGuestImage,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-qcow2",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesImageInstaller,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "image-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesImageInstaller,
			requestedDistros:   []string{fedora},
			expectedImageType:  "minimal-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesIotBootableContainer,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-bootable-container",
			expectedTargetName: target.TargetNameContainer,
		},
		{
			requestedImageType: ImageTypesIotCommit,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-commit",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesIotContainer,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-container",
			expectedTargetName: target.TargetNameContainer,
		},
		{
			requestedImageType: ImageTypesIotInstaller,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesIotRawImage,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-raw-xz",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesIotSimplifiedInstaller,
			requestedDistros:   []string{fedora},
			expectedImageType:  "iot-simplified-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesLiveInstaller,
			requestedDistros:   []string{fedora},
			expectedImageType:  "workstation-live-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesMinimalRaw,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "minimal-raw",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesMinimalRaw,
			requestedDistros:   []string{fedora},
			expectedImageType:  "minimal-raw-xz",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesOci,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "oci",
			expectedTargetName: target.TargetNameOCIObjectStorage,
		},
		{
			requestedImageType: ImageTypesOci,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-oci",
			expectedTargetName: target.TargetNameOCIObjectStorage,
		},
		{
			requestedImageType: ImageTypesVsphere,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "vmdk",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesVsphere,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-vmdk",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesVsphereOva,
			requestedDistros:   []string{rhel8, centos8, rhel9, centos9},
			expectedImageType:  "ova",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesVsphereOva,
			requestedDistros:   []string{fedora},
			expectedImageType:  "server-ova",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesWsl,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "wsl",
			expectedTargetName: target.TargetNameAWSS3,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.requestedImageType), func(t *testing.T) {
			for _, d := range tt.requestedDistros {
				request := &ComposeRequest{
					Distribution: d,
					ImageRequest: &ImageRequest{
						Architecture:  "x86_64",
						ImageType:     tt.requestedImageType,
						UploadOptions: &UploadOptions{},
						Repositories: []Repository{
							Repository{
								Baseurl: common.ToPtr("http://example.org/pub/linux/repo"),
							},
						},
					},
				}
				// NOTE: repoRegistry can be nil as long as ImageRequest.Repositories has data
				got, err := request.GetImageRequests(distrofactory.NewDefault(), nil)
				assert.NoError(t, err)
				require.Len(t, got, 1)
				assert.Equal(t, tt.expectedImageType, got[0].imageType.Name())

				require.Len(t, got[0].targets, 1)
				assert.Equal(t, tt.expectedTargetName, got[0].targets[0].Name)
			}
		})
	}
}

func TestGetImageRequests_NoRepositories(t *testing.T) {
	request := &ComposeRequest{
		Distribution: TEST_DISTRO_NAME,
		ImageRequest: &ImageRequest{
			Architecture:  "x86_64",
			ImageType:     ImageTypesAws,
			UploadOptions: &UploadOptions{},
			Repositories:  []Repository{},
		},
	}
	rr, err := reporegistry.New(nil, []fs.FS{repos.FS})
	require.NoError(t, err)
	got, err := request.GetImageRequests(distrofactory.NewDefault(), rr)
	assert.NoError(t, err)
	require.Len(t, got, 1)
	require.Greater(t, len(got[0].repositories), 0)
	assert.Contains(t, got[0].repositories[0].Metalink, TEST_DISTRO_VERSION)
}

// TestGetImageRequests_BlueprintDistro test to make sure blueprint distro overrides request distro
func TestGetImageRequests_BlueprintDistro(t *testing.T) {
	request := &ComposeRequest{
		Distribution: TEST_DISTRO_NAME,
		ImageRequest: &ImageRequest{
			Architecture:  "x86_64",
			ImageType:     ImageTypesAws,
			UploadOptions: &UploadOptions{},
			Repositories:  []Repository{},
		},
		Blueprint: &Blueprint{
			Name:   "distro-test",
			Distro: common.ToPtr(TEST_DISTRO_NAME),
		},
	}
	rr, err := reporegistry.New(nil, []fs.FS{repos.FS})
	require.NoError(t, err)
	got, err := request.GetImageRequests(distrofactory.NewDefault(), rr)
	assert.NoError(t, err)
	require.Len(t, got, 1)
	require.Greater(t, len(got[0].repositories), 0)
	assert.Contains(t, got[0].repositories[0].Metalink, TEST_DISTRO_VERSION)
	assert.Equal(t, got[0].blueprint.Distro, TEST_DISTRO_NAME)
}

func TestOpenSCAPTailoringOptions(t *testing.T) {
	cr := ComposeRequest{
		Customizations: &Customizations{
			Openscap: &OpenSCAP{
				ProfileId: "test-123",
				Tailoring: &OpenSCAPTailoring{
					Selected:   common.ToPtr([]string{"one", "two", "three"}),
					Unselected: common.ToPtr([]string{"four", "five", "six"}),
				},
			},
		},
	}

	expectedOscap := &blueprint.OpenSCAPCustomization{
		ProfileID: "test-123",
		Tailoring: &blueprint.OpenSCAPTailoringCustomizations{
			Selected:   []string{"one", "two", "three"},
			Unselected: []string{"four", "five", "six"},
		},
	}

	bp, err := cr.GetBlueprintFromCustomizations()
	assert.NoError(t, err)
	assert.Equal(t, expectedOscap, bp.Customizations.OpenSCAP)

	cr = ComposeRequest{
		Customizations: &Customizations{
			Openscap: &OpenSCAP{
				ProfileId: "test-123",
				JsonTailoring: &OpenSCAPJSONTailoring{
					ProfileId: "test-123-tailoring",
					Filepath:  "/some/filepath",
				},
			},
		},
	}

	expectedOscap = &blueprint.OpenSCAPCustomization{
		ProfileID: "test-123",
		JSONTailoring: &blueprint.OpenSCAPJSONTailoringCustomizations{
			ProfileID: "test-123-tailoring",
			Filepath:  "/some/filepath",
		},
	}

	bp, err = cr.GetBlueprintFromCustomizations()
	assert.NoError(t, err)
	assert.Equal(t, expectedOscap, bp.Customizations.OpenSCAP)

	cr = ComposeRequest{
		Customizations: &Customizations{
			Openscap: &OpenSCAP{
				ProfileId: "test-123",
				Tailoring: &OpenSCAPTailoring{
					Selected:   common.ToPtr([]string{"one", "two", "three"}),
					Unselected: common.ToPtr([]string{"four", "five", "six"}),
				},
				JsonTailoring: &OpenSCAPJSONTailoring{
					ProfileId: "test-123-tailoring",
					Filepath:  "/some/filepath",
				},
			},
		},
	}

	bp, err = cr.GetBlueprintFromCustomizations()
	assert.Error(t, err)
}

func TestDecodeMinsize(t *testing.T) {
	type testCase struct {
		in           *Minsize
		expOut       uint64
		expErrSubstr string
	}

	msStr := func(s string) *Minsize {
		var ms Minsize
		if err := ms.FromMinsize1(s); err != nil {
			panic(err)
		}
		return &ms
	}

	msInt := func(i uint64) *Minsize {
		var ms Minsize
		if err := ms.FromMinsize0(i); err != nil {
			panic(err)
		}
		return &ms
	}

	testCases := []testCase{
		{
			in:     nil,
			expOut: 0,
		},
		{
			in:     msInt(10),
			expOut: 10,
		},
		{
			in:     msInt(41 * datasizes.MiB),
			expOut: 41 * datasizes.MiB,
		},
		{
			in:     msStr("10"),
			expOut: 10,
		},
		{
			in:     msStr("32 GiB"),
			expOut: 32 * datasizes.GiB,
		},

		{
			in:           msStr("not a number"),
			expErrSubstr: "the size string doesn't contain any number: not a number",
		},
		{
			in:           msStr("10 GiBi"),
			expErrSubstr: "unknown data size units in string: 10 GiBi",
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case-%02d", idx), func(t *testing.T) {
			assert := assert.New(t)
			out, err := decodeMinsize(tc.in)
			if tc.expErrSubstr != "" {
				assert.ErrorContains(err, tc.expErrSubstr)
			} else {
				assert.NoError(err)
			}
			assert.Equal(tc.expOut, out)
		})
	}
}
