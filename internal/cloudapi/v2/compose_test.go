package v2

import (
	"testing"

	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// interface{} is a terrible idea. Work around it...
	var rootStr interface{} = "root"

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
				User:          &rootStr,
				Group:         &rootStr,
				Mode:          common.ToPtr("0755"),
			},
		},
		Files: &[]File{
			File{
				Path:          "/etc/mad.conf",
				Data:          common.ToPtr("Alfred E. Neuman was here.\n"),
				EnsureParents: common.ToPtr(true),
				User:          &rootStr,
				Group:         &rootStr,
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

	// interface{} is a terrible idea. Work around it...
	var rootStr interface{} = "root"

	// Construct the compose request with a blueprint
	cr = ComposeRequest{Blueprint: &Blueprint{
		Name:     "empty blueprint",
		Version:  common.ToPtr("0.0.0"),
		Packages: &[]Package{{Name: "bash"}, {Name: "tmux"}},
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
					User:          &rootStr,
					Group:         &rootStr,
					Mode:          common.ToPtr("0755"),
				},
			},
			Files: &[]BlueprintFile{
				{
					Path:  "/etc/mad.conf",
					Data:  common.ToPtr("Alfred E. Neuman was here.\n"),
					User:  &rootStr,
					Group: &rootStr,
					Mode:  common.ToPtr("0644"),
				},
			},
			Filesystem: &[]BlueprintFilesystem{
				{
					Mountpoint: "/var/lib/wopr/",
					Minsize:    1099511627776,
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
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "ami",
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
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "vhd",
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
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "qcow2",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesImageInstaller,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "image-installer",
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
			expectedImageType:  "iot-raw-image",
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
			expectedImageType:  "live-installer",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesMinimalRaw,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "minimal-raw",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesOci,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "oci",
			expectedTargetName: target.TargetNameOCIObjectStorage,
		},
		{
			requestedImageType: ImageTypesVsphere,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "vmdk",
			expectedTargetName: target.TargetNameAWSS3,
		},
		{
			requestedImageType: ImageTypesVsphereOva,
			requestedDistros:   []string{fedora, rhel8, centos8, rhel9, centos9},
			expectedImageType:  "ova",
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
				uo := UploadOptions(struct{}{})
				request := &ComposeRequest{
					Distribution: d,
					ImageRequest: &ImageRequest{
						Architecture:  "x86_64",
						ImageType:     tt.requestedImageType,
						UploadOptions: &uo,
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
	uo := UploadOptions(struct{}{})
	request := &ComposeRequest{
		Distribution: "fedora-40",
		ImageRequest: &ImageRequest{
			Architecture:  "x86_64",
			ImageType:     ImageTypesAws,
			UploadOptions: &uo,
			Repositories:  []Repository{},
		},
	}
	// NOTE: current directory is the location of this file, back up so it can use ./repositories/
	rr, err := reporegistry.New([]string{"../../../"})
	require.NoError(t, err)
	got, err := request.GetImageRequests(distrofactory.NewDefault(), rr)
	assert.NoError(t, err)
	require.Len(t, got, 1)
	require.Greater(t, len(got[0].repositories), 0)
	assert.Contains(t, got[0].repositories[0].Metalink, "40")
}

// TestGetImageRequests_BlueprintDistro test to make sure blueprint distro overrides request distro
func TestGetImageRequests_BlueprintDistro(t *testing.T) {
	uo := UploadOptions(struct{}{})
	request := &ComposeRequest{
		Distribution: "fedora-40",
		ImageRequest: &ImageRequest{
			Architecture:  "x86_64",
			ImageType:     ImageTypesAws,
			UploadOptions: &uo,
			Repositories:  []Repository{},
		},
		Blueprint: &Blueprint{
			Name:   "distro-test",
			Distro: common.ToPtr("fedora-39"),
		},
	}
	// NOTE: current directory is the location of this file, back up so it can use ./repositories/
	rr, err := reporegistry.New([]string{"../../../"})
	require.NoError(t, err)
	got, err := request.GetImageRequests(distrofactory.NewDefault(), rr)
	assert.NoError(t, err)
	require.Len(t, got, 1)
	require.Greater(t, len(got[0].repositories), 0)
	assert.Contains(t, got[0].repositories[0].Metalink, "39")
	assert.Equal(t, got[0].blueprint.Distro, "fedora-39")
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
