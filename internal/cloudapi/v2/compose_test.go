package v2

import (
	"testing"

	"github.com/osbuild/images/pkg/subscription"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBlueprintWithCustomizations(t *testing.T) {
	// Empty request should return empty blueprint
	cr := ComposeRequest{}
	bp, err := cr.GetBlueprintWithCustomizations()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	// Empty request should return empty blueprint
	cr = ComposeRequest{
		Customizations: &Customizations{},
	}
	bp, err = cr.GetBlueprintWithCustomizations()
	require.Nil(t, err)
	assert.Equal(t, "empty blueprint", bp.Name)
	assert.Equal(t, "0.0.0", bp.Version)
	assert.Nil(t, bp.Customizations)

	// Test with customizations
	expected := blueprint.Blueprint{Name: "empty blueprint"}
	err = expected.Initialize()
	require.Nil(t, err)

	// interface{} is a terrible idea. Work around it...
	var rootStr interface{} = "root"

	// anonymous structs buried in a data type are almost as bad.
	services := struct {
		Disabled *[]string `json:"disabled,omitempty"`
		Enabled  *[]string `json:"enabled,omitempty"`
	}{
		Disabled: &[]string{"cleanup"},
		Enabled:  &[]string{"sshd"},
	}

	// Construct the compose request with customizations
	cr = ComposeRequest{Customizations: &Customizations{
		Users: &[]User{
			User{
				Name:   "admin",
				Key:    common.ToPtr("dummy ssh-key"),
				Groups: &[]string{"users", "wheel"},
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
		Services: &services,
		Openscap: &OpenSCAP{ProfileId: "B 263-59"},
		CustomRepositories: &[]CustomRepository{
			CustomRepository{
				Id:       "custom repo",
				Metalink: common.ToPtr("http://example.org/metalink"),
				CheckGpg: common.ToPtr(true),
				Enabled:  common.ToPtr(true),
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
			DiunPubKeyHash:         common.ToPtr("pubkeyhash"),
			DiunPubKeyInsecure:     common.ToPtr("pubkeyinsecure"),
			DiunPubKeyRootCerts:    common.ToPtr("pubkeyrootcerts"),
			ManufacturingServerUrl: common.ToPtr("serverurl"),
		},
		Ignition: &Ignition{
			Firstboot: &IgnitionFirstboot{
				Url: "provisioning-url.local",
			},
		},
		Sshkey: &[]SSHKey{
			{
				Key:  "key",
				User: "user",
			},
		},
		Timezone: &Timezone{
			Timezone: common.ToPtr("US/Eastern"),
			Ntpservers: common.ToPtr([]string{
				"0.north-america.pool.ntp.org",
				"1.north-america.pool.ntp.org",
			}),
		},
	}}

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
				Name:   "admin",
				Key:    common.ToPtr("dummy ssh-key"),
				Groups: []string{"users", "wheel"},
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
		},
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID: "B 263-59",
		},
		Repositories: []blueprint.RepositoryCustomization{
			blueprint.RepositoryCustomization{
				Id:       "custom repo",
				Metalink: "http://example.org/metalink",
				Enabled:  common.ToPtr(true),
				GPGCheck: common.ToPtr(true),
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
			DiunPubKeyHash:         "pubkeyhash",
			DiunPubKeyInsecure:     "pubkeyinsecure",
			DiunPubKeyRootCerts:    "pubkeyrootcerts",
			ManufacturingServerURL: "serverurl",
		},
		Ignition: &blueprint.IgnitionCustomization{
			FirstBoot: &blueprint.FirstBootIgnitionCustomization{
				ProvisioningURL: "provisioning-url.local",
			},
		},
		SSHKey: []blueprint.SSHKeyCustomization{
			{
				Key:  "key",
				User: "user",
			},
		},
		Timezone: &blueprint.TimezoneCustomization{
			Timezone: common.ToPtr("US/Eastern"),
			NTPServers: []string{
				"0.north-america.pool.ntp.org",
				"1.north-america.pool.ntp.org",
			},
		},
	}
	bp, err = cr.GetBlueprintWithCustomizations()
	require.Nil(t, err)
	assert.Equal(t, bp, expected)
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
				Baseurl:     common.ToPtr("http://example.org/pub/linux/repo"),
				CheckGpg:    common.ToPtr(true),
				PackageSets: &[]string{"build", "archive"},
				Rhsm:        common.ToPtr(false),
			},
		},
	}}

	expected := []Repository{
		Repository{
			Baseurl:     common.ToPtr("http://example.org/pub/linux/repo"),
			CheckGpg:    common.ToPtr(true),
			PackageSets: &[]string{"build", "archive"},
			Rhsm:        common.ToPtr(false),
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
