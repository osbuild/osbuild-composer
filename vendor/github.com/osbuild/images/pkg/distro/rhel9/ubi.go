package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

var wslImgType = imageType{
	name:     "wsl",
	filename: "disk.tar.gz",
	mimeType: "application/x-tar",
	packageSets: map[string]packageSetFunc{
		osPkgsKey: ubiCommonPackageSet,
	},
	defaultImageConfig: &distro.ImageConfig{
		Locale:    common.ToPtr("en_US.UTF-8"),
		NoSElinux: common.ToPtr(true),
		WSLConfig: &osbuild.WSLConfStageOptions{
			Boot: osbuild.WSLConfBootOptions{
				Systemd: true,
			},
		},
	},
	bootable:         false,
	image:            tarImage,
	buildPipelines:   []string{"build"},
	payloadPipelines: []string{"os", "archive"},
	exports:          []string{"archive"},
}

func ubiCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"alternatives",
			"audit-libs",
			"basesystem",
			"bash",
			"ca-certificates",
			"coreutils-single",
			"crypto-policies-scripts",
			"curl-minimal",
			"dejavu-sans-fonts",
			"dnf",
			"filesystem",
			"findutils",
			"gdb-gdbserver",
			// Differs from official UBI, as we don't include CRB repos
			// "gdbm",
			"glibc-minimal-langpack",
			"gmp",
			"gnupg2",
			"gobject-introspection",
			"hostname",
			"langpacks-en",
			"libcurl-minimal",
			"openssl",
			"pam",
			"passwd",
			"procps-ng",
			"python3",
			"python3-inotify",
			"redhat-release",
			"rootfiles",
			"rpm",
			"sed",
			"setup",
			"shadow-utils",
			"subscription-manager",
			"systemd",
			"tar",
			"tpm2-tss",
			"tzdata",
			"util-linux",
			"vim-minimal",
			"yum",
		},
		Exclude: []string{
			"gawk-all-langpacks",
			"glibc-gconv-extra",
			"glibc-langpack-en",
			"openssl-pkcs11",
			"python-unversioned-command",
			"redhat-release-eula",
			"rpm-plugin-systemd-inhibit",
		},
	}

	return ps
}
