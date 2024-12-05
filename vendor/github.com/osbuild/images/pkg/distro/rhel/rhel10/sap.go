package rhel10

import (
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

// sapImageConfig returns the SAP specific ImageConfig data
func sapImageConfig(osVersion string) *distro.ImageConfig {
	return &distro.ImageConfig{
		SELinuxConfig: &osbuild.SELinuxConfigStageOptions{
			State: osbuild.SELinuxStatePermissive,
		},
		// RHBZ#1960617
		Tuned: osbuild.NewTunedStageOptions("sap-hana"),
		// RHBZ#1959979
		Tmpfilesd: []*osbuild.TmpfilesdStageOptions{
			osbuild.NewTmpfilesdStageOptions("sap.conf",
				[]osbuild.TmpfilesdConfigLine{
					{
						Type: "x",
						Path: "/tmp/.sap*",
					},
					{
						Type: "x",
						Path: "/tmp/.hdb*lock",
					},
					{
						Type: "x",
						Path: "/tmp/.trex*lock",
					},
				},
			),
		},
		// RHBZ#1959963
		PamLimitsConf: []*osbuild.PamLimitsConfStageOptions{
			osbuild.NewPamLimitsConfStageOptions("99-sap.conf",
				[]osbuild.PamLimitsConfigLine{
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
				},
			),
		},
		// RHBZ#1959962
		Sysctld: []*osbuild.SysctldStageOptions{
			osbuild.NewSysctldStageOptions("sap.conf",
				[]osbuild.SysctldConfigLine{
					{
						Key:   "kernel.pid_max",
						Value: "4194304",
					},
					{
						Key:   "vm.max_map_count",
						Value: "2147483647",
					},
				},
			),
		},
		// E4S/EUS
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			osbuild.NewDNFConfigStageOptions(
				[]osbuild.DNFVariable{
					{
						Name:  "releasever",
						Value: osVersion,
					},
				},
				nil,
			),
		},
	}
}

func SapPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			// RHBZ#2076763
			"@Server",
			// SAP System Roles
			// https://access.redhat.com/sites/default/files/attachments/rhel_system_roles_for_sap_1.pdf
			"ansible-core",
			"rhel-system-roles-sap",
			// RHBZ#1959813
			"bind-utils",
			"nfs-utils",
			"tcsh",
			// RHBZ#1959955
			"uuidd",
			// RHBZ#1959923
			"cairo",
			"expect",
			"graphviz",
			//"gtk2", // gtk2 is not available in RHEL-10
			"iptraf-ng",
			"krb5-workstation",
			"libaio",
			"libatomic",
			"libicu",
			"libtool-ltdl",
			"lm_sensors",
			"net-tools",
			"numactl",
			"PackageKit-gtk3-module",
			"xorg-x11-xauth",
			// RHBZ#1960617
			"tuned-profiles-sap-hana",
			// RHBZ#1961168
			"libnsl",
		},
		Exclude: []string{
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
		},
	}
}
