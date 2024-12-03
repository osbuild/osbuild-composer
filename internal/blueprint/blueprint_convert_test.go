package blueprint

import (
	"testing"

	iblueprint "github.com/osbuild/images/pkg/blueprint"
	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		src      Blueprint
		expected iblueprint.Blueprint
	}{
		{
			name:     "empty",
			src:      Blueprint{},
			expected: iblueprint.Blueprint{},
		},
		{
			name: "everything",
			src: Blueprint{
				Name:        "name",
				Description: "desc",
				Version:     "version",
				Packages: []Package{
					{
						Name:    "package-name",
						Version: "package-version",
					},
				},
				Modules: []Package{
					{
						Name:    "module-name",
						Version: "module-version",
					},
				},
				Groups: []Group{
					{
						Name: "group-name",
					},
				},
				Containers: []Container{
					{
						Source:    "source",
						Name:      "name",
						TLSVerify: common.ToPtr(true),
					},
				},
				Customizations: &Customizations{
					Hostname: common.ToPtr("hostname"),
					Kernel: &KernelCustomization{
						Name:   "kernel-name",
						Append: "kernel-append",
					},
					SSHKey: []SSHKeyCustomization{
						{
							User: "ssh-user",
							Key:  "ssh-key",
						},
					},
					User: []UserCustomization{
						{
							Name:        "user-name",
							Description: common.ToPtr("user-desc"),
							Password:    common.ToPtr("user-password"),
							Key:         common.ToPtr("user-key"),
							Home:        common.ToPtr("/home/user"),
							Shell:       common.ToPtr("fish"),
							Groups:      []string{"wheel"},
							UID:         common.ToPtr(42),
							GID:         common.ToPtr(2023),
						},
					},
					Group: []GroupCustomization{
						{
							Name: "group",
							GID:  common.ToPtr(7),
						},
					},
					Timezone: &TimezoneCustomization{
						Timezone:   common.ToPtr("timezone"),
						NTPServers: []string{"ntp-server"},
					},
					Locale: &LocaleCustomization{
						Languages: []string{"language"},
						Keyboard:  common.ToPtr("keyboard"),
					},
					Firewall: &FirewallCustomization{
						Ports: []string{"80"},
						Services: &FirewallServicesCustomization{
							Enabled:  []string{"ssh"},
							Disabled: []string{"ntp"},
						},
						Zones: []FirewallZoneCustomization{
							{
								Name:    common.ToPtr("name"),
								Sources: []string{"src"},
							},
						},
					},
					Services: &ServicesCustomization{
						Enabled:  []string{"osbuild-composer.service"},
						Disabled: []string{"lorax-composer.service"},
					},
					Filesystem: []FilesystemCustomization{
						{
							Mountpoint: "/usr",
							MinSize:    1024,
						},
					},
					Disk: &DiskCustomization{
						MinSize: 10240,
						Partitions: []PartitionCustomization{
							{
								// this partition is invalid, since only one of
								// btrfs, vg, or filesystem should be set, but
								// the converter copies everything
								// unconditionally, so let's test the full
								// thing
								Type:    "plain",
								MinSize: 1024,
								BtrfsVolumeCustomization: BtrfsVolumeCustomization{
									Subvolumes: []BtrfsSubvolumeCustomization{
										{
											Name:       "subvol1",
											Mountpoint: "/subvol1",
										},
										{
											Name:       "subvol2",
											Mountpoint: "/subvol2",
										},
									},
								},
								VGCustomization: VGCustomization{
									Name: "vg1",
									LogicalVolumes: []LVCustomization{
										{
											Name:    "vg1lv1",
											MinSize: 0,
											FilesystemTypedCustomization: FilesystemTypedCustomization{
												Mountpoint: "/one",
												Label:      "one",
												FSType:     "xfs",
											},
										},
										{
											Name:    "vg1lv2",
											MinSize: 0,
											FilesystemTypedCustomization: FilesystemTypedCustomization{
												Mountpoint: "/two",
												Label:      "two",
												FSType:     "ext4",
											},
										},
									},
								},
								FilesystemTypedCustomization: FilesystemTypedCustomization{
									Mountpoint: "/root",
									Label:      "roothome",
									FSType:     "xfs",
								},
							},
							{
								Type:    "plain",
								MinSize: 1024,
								FilesystemTypedCustomization: FilesystemTypedCustomization{
									Mountpoint: "/root",
									Label:      "roothome",
									FSType:     "xfs",
								},
							},
							{
								Type:    "lvm",
								MinSize: 1024,
								VGCustomization: VGCustomization{
									Name: "vg1",
									LogicalVolumes: []LVCustomization{
										{
											Name:    "vg1lv1",
											MinSize: 0,
											FilesystemTypedCustomization: FilesystemTypedCustomization{
												Mountpoint: "/one",
												Label:      "one",
												FSType:     "xfs",
											},
										},
										{
											Name:    "vg1lv2",
											MinSize: 0,
											FilesystemTypedCustomization: FilesystemTypedCustomization{
												Mountpoint: "/two",
												Label:      "two",
												FSType:     "ext4",
											},
										},
									},
								},
							},
							{
								Type:    "btrfs",
								MinSize: 1024,
								BtrfsVolumeCustomization: BtrfsVolumeCustomization{
									Subvolumes: []BtrfsSubvolumeCustomization{
										{
											Name:       "subvol1",
											Mountpoint: "/subvol1",
										},
										{
											Name:       "subvol2",
											Mountpoint: "/subvol2",
										},
									},
								},
							},
						},
					},
					InstallationDevice: "/dev/sda",
					FDO: &FDOCustomization{
						ManufacturingServerURL:  "http://manufacturing.fdo",
						DiunPubKeyInsecure:      "insecure-pubkey",
						DiunPubKeyHash:          "hash-pubkey",
						DiunPubKeyRootCerts:     "root-certs",
						DiMfgStringTypeMacIface: "iface",
					},
					OpenSCAP: &OpenSCAPCustomization{
						DataStream: "stream",
						ProfileID:  "profile",
						Tailoring: &OpenSCAPTailoringCustomizations{
							Selected:   []string{"cloth"},
							Unselected: []string{"leather"},
						},
					},
					Ignition: &IgnitionCustomization{
						Embedded: &EmbeddedIgnitionCustomization{
							Config: "ignition-config",
						},
						FirstBoot: &FirstBootIgnitionCustomization{
							ProvisioningURL: "http://provisioning.edge",
						},
					},
					Directories: []DirectoryCustomization{
						{
							Path:          "/dir",
							User:          common.ToPtr("dir-user"),
							Group:         common.ToPtr("dir-group"),
							Mode:          "0777",
							EnsureParents: true,
						},
					},
					Files: []FileCustomization{
						{
							Path:  "/file",
							User:  common.ToPtr("file-user`"),
							Group: common.ToPtr("file-group"),
							Mode:  "0755",
							Data:  "literal easter egg",
						},
					},
					Repositories: []RepositoryCustomization{
						{
							Id:           "repoid",
							BaseURLs:     []string{"http://baseurl"},
							GPGKeys:      []string{"repo-gpgkey"},
							Metalink:     "http://metalink",
							Mirrorlist:   "http://mirrorlist",
							Name:         "reponame",
							Priority:     common.ToPtr(987),
							Enabled:      common.ToPtr(true),
							GPGCheck:     common.ToPtr(true),
							RepoGPGCheck: common.ToPtr(true),
							SSLVerify:    common.ToPtr(true),
							Filename:     "repofile",
						},
					},
					Installer: &InstallerCustomization{
						Unattended:   true,
						SudoNopasswd: []string{"%group", "user"},
						Kickstart: &Kickstart{
							Contents: "# test kickstart addition created by osbuild-composer",
						},
						Modules: &AnacondaModules{
							Enable: []string{
								"org.fedoraproject.Anaconda.Modules.Localization",
								"org.fedoraproject.Anaconda.Modules.Users",
							},
							Disable: []string{
								"org.fedoraproject.Anaconda.Modules.Network",
							},
						},
					},
					RPM: &RPMCustomization{
						ImportKeys: &RPMImportKeys{
							Files: []string{"/root/gpg-key"},
						},
					},
					RHSM: &RHSMCustomization{
						Config: &RHSMConfig{
							DNFPlugins: &SubManDNFPluginsConfig{
								ProductID: &DNFPluginConfig{
									Enabled: common.ToPtr(true),
								},
								SubscriptionManager: &DNFPluginConfig{
									Enabled: common.ToPtr(false),
								},
							},
							SubscriptionManager: &SubManConfig{
								RHSMConfig: &SubManRHSMConfig{
									ManageRepos: common.ToPtr(true),
								},
								RHSMCertdConfig: &SubManRHSMCertdConfig{
									AutoRegistration: common.ToPtr(false),
								},
							},
						},
					},
					CACerts: &CACustomization{
						PEMCerts: []string{"pem-cert"},
					},
				},
				Distro: "distro",
			},
			expected: iblueprint.Blueprint{
				Name:        "name",
				Description: "desc",
				Version:     "version",
				Packages: []iblueprint.Package{
					{
						Name:    "package-name",
						Version: "package-version",
					},
				},
				Modules: []iblueprint.Package{
					{
						Name:    "module-name",
						Version: "module-version",
					},
				},
				Groups: []iblueprint.Group{
					{
						Name: "group-name",
					},
				},
				Containers: []iblueprint.Container{
					{
						Source:    "source",
						Name:      "name",
						TLSVerify: common.ToPtr(true),
					},
				},
				Customizations: &iblueprint.Customizations{
					Hostname: common.ToPtr("hostname"),
					Kernel: &iblueprint.KernelCustomization{
						Name:   "kernel-name",
						Append: "kernel-append",
					},
					User: []iblueprint.UserCustomization{
						{
							Name: "ssh-user", // converted from sshkey
							Key:  common.ToPtr("ssh-key"),
						},
						{
							Name:        "user-name",
							Description: common.ToPtr("user-desc"),
							Password:    common.ToPtr("user-password"),
							Key:         common.ToPtr("user-key"),
							Home:        common.ToPtr("/home/user"),
							Shell:       common.ToPtr("fish"),
							Groups:      []string{"wheel"},
							UID:         common.ToPtr(42),
							GID:         common.ToPtr(2023),
						},
					},
					Group: []iblueprint.GroupCustomization{
						{
							Name: "group",
							GID:  common.ToPtr(7),
						},
					},
					Timezone: &iblueprint.TimezoneCustomization{
						Timezone:   common.ToPtr("timezone"),
						NTPServers: []string{"ntp-server"},
					},
					Locale: &iblueprint.LocaleCustomization{
						Languages: []string{"language"},
						Keyboard:  common.ToPtr("keyboard"),
					},
					Firewall: &iblueprint.FirewallCustomization{
						Ports: []string{"80"},
						Services: &iblueprint.FirewallServicesCustomization{
							Enabled:  []string{"ssh"},
							Disabled: []string{"ntp"},
						},
						Zones: []iblueprint.FirewallZoneCustomization{
							{
								Name:    common.ToPtr("name"),
								Sources: []string{"src"},
							},
						},
					},
					Services: &iblueprint.ServicesCustomization{
						Enabled:  []string{"osbuild-composer.service"},
						Disabled: []string{"lorax-composer.service"},
					},
					Filesystem: []iblueprint.FilesystemCustomization{
						{
							Mountpoint: "/usr",
							MinSize:    1024,
						},
					},
					Disk: &iblueprint.DiskCustomization{
						MinSize: 10240,
						Partitions: []iblueprint.PartitionCustomization{
							{
								// this partition is invalid, since only one of
								// btrfs, vg, or filesystem should be set, but
								// the converter copies everything
								// unconditionally, so let's test the full
								// thing
								Type:    "plain",
								MinSize: 1024,
								BtrfsVolumeCustomization: iblueprint.BtrfsVolumeCustomization{
									Subvolumes: []iblueprint.BtrfsSubvolumeCustomization{
										{
											Name:       "subvol1",
											Mountpoint: "/subvol1",
										},
										{
											Name:       "subvol2",
											Mountpoint: "/subvol2",
										},
									},
								},
								VGCustomization: iblueprint.VGCustomization{
									Name: "vg1",
									LogicalVolumes: []iblueprint.LVCustomization{
										{
											Name:    "vg1lv1",
											MinSize: 0,
											FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
												Mountpoint: "/one",
												Label:      "one",
												FSType:     "xfs",
											},
										},
										{
											Name:    "vg1lv2",
											MinSize: 0,
											FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
												Mountpoint: "/two",
												Label:      "two",
												FSType:     "ext4",
											},
										},
									},
								},
								FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
									Mountpoint: "/root",
									Label:      "roothome",
									FSType:     "xfs",
								},
							},
							{
								Type:    "plain",
								MinSize: 1024,
								FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
									Mountpoint: "/root",
									Label:      "roothome",
									FSType:     "xfs",
								},
							},
							{
								Type:    "lvm",
								MinSize: 1024,
								VGCustomization: iblueprint.VGCustomization{
									Name: "vg1",
									LogicalVolumes: []iblueprint.LVCustomization{
										{
											Name:    "vg1lv1",
											MinSize: 0,
											FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
												Mountpoint: "/one",
												Label:      "one",
												FSType:     "xfs",
											},
										},
										{
											Name:    "vg1lv2",
											MinSize: 0,
											FilesystemTypedCustomization: iblueprint.FilesystemTypedCustomization{
												Mountpoint: "/two",
												Label:      "two",
												FSType:     "ext4",
											},
										},
									},
								},
							},
							{
								Type:    "btrfs",
								MinSize: 1024,
								BtrfsVolumeCustomization: iblueprint.BtrfsVolumeCustomization{
									Subvolumes: []iblueprint.BtrfsSubvolumeCustomization{
										{
											Name:       "subvol1",
											Mountpoint: "/subvol1",
										},
										{
											Name:       "subvol2",
											Mountpoint: "/subvol2",
										},
									},
								},
							},
						},
					},
					InstallationDevice: "/dev/sda",
					FDO: &iblueprint.FDOCustomization{
						ManufacturingServerURL:  "http://manufacturing.fdo",
						DiunPubKeyInsecure:      "insecure-pubkey",
						DiunPubKeyHash:          "hash-pubkey",
						DiunPubKeyRootCerts:     "root-certs",
						DiMfgStringTypeMacIface: "iface",
					},
					OpenSCAP: &iblueprint.OpenSCAPCustomization{
						DataStream: "stream",
						ProfileID:  "profile",
						Tailoring: &iblueprint.OpenSCAPTailoringCustomizations{
							Selected:   []string{"cloth"},
							Unselected: []string{"leather"},
						},
					},
					Ignition: &iblueprint.IgnitionCustomization{
						Embedded: &iblueprint.EmbeddedIgnitionCustomization{
							Config: "ignition-config",
						},
						FirstBoot: &iblueprint.FirstBootIgnitionCustomization{
							ProvisioningURL: "http://provisioning.edge",
						},
					},
					Directories: []iblueprint.DirectoryCustomization{
						{
							Path:          "/dir",
							User:          common.ToPtr("dir-user"),
							Group:         common.ToPtr("dir-group"),
							Mode:          "0777",
							EnsureParents: true,
						},
					},
					Files: []iblueprint.FileCustomization{
						{
							Path:  "/file",
							User:  common.ToPtr("file-user`"),
							Group: common.ToPtr("file-group"),
							Mode:  "0755",
							Data:  "literal easter egg",
						},
					},
					Repositories: []iblueprint.RepositoryCustomization{
						{
							Id:           "repoid",
							BaseURLs:     []string{"http://baseurl"},
							GPGKeys:      []string{"repo-gpgkey"},
							Metalink:     "http://metalink",
							Mirrorlist:   "http://mirrorlist",
							Name:         "reponame",
							Priority:     common.ToPtr(987),
							Enabled:      common.ToPtr(true),
							GPGCheck:     common.ToPtr(true),
							RepoGPGCheck: common.ToPtr(true),
							SSLVerify:    common.ToPtr(true),
							Filename:     "repofile",
						},
					},
					Installer: &iblueprint.InstallerCustomization{
						Unattended:   true,
						SudoNopasswd: []string{"%group", "user"},
						Kickstart: &iblueprint.Kickstart{
							Contents: "# test kickstart addition created by osbuild-composer",
						},
						Modules: &iblueprint.AnacondaModules{
							Enable: []string{
								"org.fedoraproject.Anaconda.Modules.Localization",
								"org.fedoraproject.Anaconda.Modules.Users",
							},
							Disable: []string{
								"org.fedoraproject.Anaconda.Modules.Network",
							},
						},
					},
					RPM: &iblueprint.RPMCustomization{
						ImportKeys: &iblueprint.RPMImportKeys{
							Files: []string{"/root/gpg-key"},
						},
					},
					RHSM: &iblueprint.RHSMCustomization{
						Config: &iblueprint.RHSMConfig{
							DNFPlugins: &iblueprint.SubManDNFPluginsConfig{
								ProductID: &iblueprint.DNFPluginConfig{
									Enabled: common.ToPtr(true),
								},
								SubscriptionManager: &iblueprint.DNFPluginConfig{
									Enabled: common.ToPtr(false),
								},
							},
							SubscriptionManager: &iblueprint.SubManConfig{
								RHSMConfig: &iblueprint.SubManRHSMConfig{
									ManageRepos: common.ToPtr(true),
								},
								RHSMCertdConfig: &iblueprint.SubManRHSMCertdConfig{
									AutoRegistration: common.ToPtr(false),
								},
							},
						},
					},
					CACerts: &iblueprint.CACustomization{
						PEMCerts: []string{"pem-cert"},
					},
				},
				Distro: "distro",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Convert(tt.src))
		})
	}
}
