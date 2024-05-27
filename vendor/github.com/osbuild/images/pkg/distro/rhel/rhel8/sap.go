package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

// sapImageConfig returns the SAP specific ImageConfig data
func sapImageConfig(rd distro.Distro) *distro.ImageConfig {
	ic := &distro.ImageConfig{
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
	}

	if common.VersionLessThan(rd.OsVersion(), "8.10") {
		// E4S/EUS
		ic.DNFConfig = []*osbuild.DNFConfigStageOptions{
			osbuild.NewDNFConfigStageOptions(
				[]osbuild.DNFVariable{
					{
						Name:  "releasever",
						Value: rd.OsVersion(),
					},
				},
				nil,
			),
		}
	}

	return ic
}

func SapPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	packageSet := rpmmd.PackageSet{
		Include: []string{
			// RHBZ#2074107
			"@Server",
			// SAP System Roles
			// https://access.redhat.com/sites/default/files/attachments/rhel_system_roles_for_sap_1.pdf
			"rhel-system-roles-sap",
			// RHBZ#1959813
			"bind-utils",
			"compat-sap-c++-9",
			"compat-sap-c++-10", // RHBZ#2074114
			"nfs-utils",
			"tcsh",
			// RHBZ#1959955
			"uuidd",
			// RHBZ#1959923
			"cairo",
			"expect",
			"graphviz",
			"gtk2",
			"iptraf-ng",
			"krb5-workstation",
			"libaio",
			"libatomic",
			"libcanberra-gtk2",
			"libicu",
			"libpng12",
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
	}

	if common.VersionLessThan(t.Arch().Distro().OsVersion(), "8.6") {
		packageSet = packageSet.Append(rpmmd.PackageSet{
			Include: []string{"ansible"},
		})
	} else {
		// 8.6+ and CS8 (image type does not exist on 8.5)
		packageSet = packageSet.Append(rpmmd.PackageSet{
			Include: []string{"ansible-core"}, // RHBZ#2077356
		})
	}
	return packageSet
}
