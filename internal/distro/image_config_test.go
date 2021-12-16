package distro

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/stretchr/testify/assert"
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
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
				Sysconfig: []*osbuild2.SysconfigStageOptions{
					{
						Kernel: &osbuild2.SysconfigKernelOptions{
							UpdateDefault: true,
							DefaultKernel: "kernel",
						},
						Network: &osbuild2.SysconfigNetworkOptions{
							Networking: true,
							NoZeroConf: true,
						},
						NetworkScripts: &osbuild2.NetworkScriptsOptions{
							IfcfgFiles: map[string]osbuild2.IfcfgFile{
								"eth0": {
									Device:    "eth0",
									Bootproto: osbuild2.IfcfgBootprotoDHCP,
									OnBoot:    common.BoolToPtr(true),
									Type:      osbuild2.IfcfgTypeEthernet,
									UserCtl:   common.BoolToPtr(true),
									PeerDNS:   common.BoolToPtr(true),
									IPv6Init:  common.BoolToPtr(false),
								},
							},
						},
					},
				},
			},
			imageConfig: &ImageConfig{
				Timezone: "UTC",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Servers: []osbuild2.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.BoolToPtr(true),
							Iburst:   common.BoolToPtr(true),
							Minpoll:  common.IntToPtr(4),
							Maxpoll:  common.IntToPtr(4),
						},
					},
					LeapsecTz: common.StringToPtr(""),
				},
			},
			expectedConfig: &ImageConfig{
				Timezone: "UTC",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Servers: []osbuild2.ChronyConfigServer{
						{
							Hostname: "169.254.169.123",
							Prefer:   common.BoolToPtr(true),
							Iburst:   common.BoolToPtr(true),
							Minpoll:  common.IntToPtr(4),
							Maxpoll:  common.IntToPtr(4),
						},
					},
					LeapsecTz: common.StringToPtr(""),
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
				Sysconfig: []*osbuild2.SysconfigStageOptions{
					{
						Kernel: &osbuild2.SysconfigKernelOptions{
							UpdateDefault: true,
							DefaultKernel: "kernel",
						},
						Network: &osbuild2.SysconfigNetworkOptions{
							Networking: true,
							NoZeroConf: true,
						},
						NetworkScripts: &osbuild2.NetworkScriptsOptions{
							IfcfgFiles: map[string]osbuild2.IfcfgFile{
								"eth0": {
									Device:    "eth0",
									Bootproto: osbuild2.IfcfgBootprotoDHCP,
									OnBoot:    common.BoolToPtr(true),
									Type:      osbuild2.IfcfgTypeEthernet,
									UserCtl:   common.BoolToPtr(true),
									PeerDNS:   common.BoolToPtr(true),
									IPv6Init:  common.BoolToPtr(false),
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
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
			imageConfig: &ImageConfig{},
			expectedConfig: &ImageConfig{
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: &ImageConfig{},
			imageConfig: &ImageConfig{
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
			expectedConfig: &ImageConfig{
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
		},
		{
			name:         "empty distro configuration",
			distroConfig: nil,
			imageConfig: &ImageConfig{
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
			expectedConfig: &ImageConfig{
				Timezone: "America/New_York",
				TimeSynchronization: &osbuild2.ChronyStageOptions{
					Timeservers: []string{"127.0.0.1"},
				},
				Locale: "en_US.UTF-8",
				Keyboard: &osbuild2.KeymapStageOptions{
					Keymap: "us",
				},
				EnabledServices:  []string{"sshd"},
				DisabledServices: []string{"named"},
				DefaultTarget:    "multi-user.target",
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectedConfig, tt.imageConfig.InheritFrom(tt.distroConfig), "test case %q failed (idx %d)", tt.name, idx)
		})
	}
}
