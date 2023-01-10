package distro

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

func TestImageConfigInheritFrom(t *testing.T) {
	tests := []struct {
		name           string
		distroConfig   *ImageConfig
		imageConfig    *ImageConfig
		expectedConfig *ImageConfig
	}{
		{
			name: "inheritance with overridden values",
			distroConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
				Sysconfig: []*osbuild.SysconfigStageOptions{
					{
						Kernel: &osbuild.SysconfigKernelOptions{
							UpdateDefault: true,
							DefaultKernel: "kernel",
						},
						Network: &osbuild.SysconfigNetworkOptions{
							Networking: true,
							NoZeroConf: true,
						},
						NetworkScripts: &osbuild.NetworkScriptsOptions{
							IfcfgFiles: map[string]osbuild.IfcfgFile{
								"eth0": {
									Device:    "eth0",
									Bootproto: osbuild.IfcfgBootprotoDHCP,
									OnBoot:    common.ToPtr(true),
									Type:      osbuild.IfcfgTypeEthernet,
									UserCtl:   common.ToPtr(true),
									PeerDNS:   common.ToPtr(true),
									IPv6Init:  common.ToPtr(false),
								},
							},
						},
					},
				},
			},
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("UTC"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.ToPtr(true),
							Iburst:   common.ToPtr(true),
							Minpoll:  common.ToPtr(4),
							Maxpoll:  common.ToPtr(4),
						},
					},
					LeapsecTz: common.ToPtr(""),
				},
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("UTC"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.ToPtr(true),
							Iburst:   common.ToPtr(true),
							Minpoll:  common.ToPtr(4),
							Maxpoll:  common.ToPtr(4),
						},
					},
					LeapsecTz: common.ToPtr(""),
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
				Sysconfig: []*osbuild.SysconfigStageOptions{
					{
						Kernel: &osbuild.SysconfigKernelOptions{
							UpdateDefault: true,
							DefaultKernel: "kernel",
						},
						Network: &osbuild.SysconfigNetworkOptions{
							Networking: true,
							NoZeroConf: true,
						},
						NetworkScripts: &osbuild.NetworkScriptsOptions{
							IfcfgFiles: map[string]osbuild.IfcfgFile{
								"eth0": {
									Device:    "eth0",
									Bootproto: osbuild.IfcfgBootprotoDHCP,
									OnBoot:    common.ToPtr(true),
									Type:      osbuild.IfcfgTypeEthernet,
									UserCtl:   common.ToPtr(true),
									PeerDNS:   common.ToPtr(true),
									IPv6Init:  common.ToPtr(false),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty image type configuration",
			distroConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			imageConfig: &ImageConfig{},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: &ImageConfig{},
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: nil,
			imageConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
			expectedConfig: &ImageConfig{
				Timezone: common.ToPtr("America/New_York"),
				TimeSynchronization: &osbuild.ChronyStageOptions{
					Servers: []osbuild.ChronyConfigServer{{Hostname: "127.0.0.1"}},
				},
				Locale: common.ToPtr("en_US.UTF-8"),
				Keyboard: &osbuild.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    common.ToPtr("multi-user.target"),
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectedConfig, tt.imageConfig.InheritFrom(tt.distroConfig), "test case %q failed (idx %d)", tt.name, idx)
		})
	}
}
