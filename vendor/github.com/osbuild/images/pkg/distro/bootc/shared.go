package bootc

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

// This file contains shared helpers between the various bootc
// image types, both here and in bootc-image-builder.
// Once the legacy bootc ISO type has moved into images we can
// unexport most (all?) of these helpers.

// TODO: find a way to move them into YAML to make sharing easier
// between package and image based image types
//
// from:https://github.com/osbuild/images/blob/v0.207.0/data/distrodefs/rhel-10/imagetypes.yaml#L169
var loraxRhelTemplates = []manifest.InstallerLoraxTemplate{
	manifest.InstallerLoraxTemplate{Path: "80-rhel/runtime-postinstall.tmpl"},
	manifest.InstallerLoraxTemplate{Path: "80-rhel/runtime-cleanup.tmpl", AfterDracut: true},
}

// from:https://github.com/osbuild/images/blob/v0.207.0/data/distrodefs/fedora/imagetypes.yaml#L408
var loraxFedoraTemplates = []manifest.InstallerLoraxTemplate{
	manifest.InstallerLoraxTemplate{Path: "99-generic/runtime-postinstall.tmpl"},
	manifest.InstallerLoraxTemplate{Path: "99-generic/runtime-cleanup.tmpl", AfterDracut: true},
}

// This will be reused by bootc-image-builder

func LoraxTemplates(si osinfo.OSRelease) []manifest.InstallerLoraxTemplate {
	switch {
	case si.ID == "rhel" || slices.Contains(si.IDLike, "rhel") || si.VersionID == "eln":
		return loraxRhelTemplates
	default:
		return loraxFedoraTemplates
	}
}
func LoraxTemplatePackage(si osinfo.OSRelease) string {
	switch {
	case si.ID == "rhel" || slices.Contains(si.IDLike, "rhel") || si.VersionID == "eln":
		return "lorax-templates-rhel"
	default:
		return "lorax-templates-generic"
	}
}

func PlatformFor(archStr, uefiVendor string) *platform.Data {
	archi := common.Must(arch.FromString(archStr))
	platform := &platform.Data{
		Arch:        archi,
		UEFIVendor:  uefiVendor,
		QCOW2Compat: "1.1",
	}
	switch archi {
	case arch.ARCH_X86_64:
		platform.BIOSPlatform = "i386-pc"
	case arch.ARCH_PPC64LE:
		platform.BIOSPlatform = "powerpc-ieee1275"
	case arch.ARCH_S390X:
		platform.ZiplSupport = true
	}
	return platform
}

func GetDistroAndRunner(osRelease osinfo.OSRelease) (manifest.Distro, runner.Runner, error) {
	switch osRelease.ID {
	case "fedora":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse Fedora version (%s): %w", osRelease.VersionID, err)
		}

		return manifest.DISTRO_FEDORA, &runner.Fedora{
			Version: version,
		}, nil
	case "centos":
		version, err := strconv.ParseUint(osRelease.VersionID, 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse CentOS version (%s): %w", osRelease.VersionID, err)
		}
		r := &runner.CentOS{
			Version: version,
		}
		switch version {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			olog.Printf("Unknown CentOS version %d, using default distro for manifest generation", version)
			return manifest.DISTRO_NULL, r, nil
		}

	case "rhel":
		versionParts := strings.Split(osRelease.VersionID, ".")
		if len(versionParts) != 2 {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("invalid RHEL version format: %s", osRelease.VersionID)
		}
		major, err := strconv.ParseUint(versionParts[0], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL major version (%s): %w", versionParts[0], err)
		}
		minor, err := strconv.ParseUint(versionParts[1], 10, 64)
		if err != nil {
			return manifest.DISTRO_NULL, nil, fmt.Errorf("cannot parse RHEL minor version (%s): %w", versionParts[1], err)
		}
		r := &runner.RHEL{
			Major: major,
			Minor: minor,
		}
		switch major {
		case 9:
			return manifest.DISTRO_EL9, r, nil
		case 10:
			return manifest.DISTRO_EL10, r, nil
		default:
			olog.Printf("Unknown RHEL version %d, using default distro for manifest generation", major)
			return manifest.DISTRO_NULL, r, nil
		}
	}

	olog.Printf("Unknown distro %s, using default runner", osRelease.ID)
	return manifest.DISTRO_NULL, &runner.Linux{}, nil
}

func NeedsRHELLoraxTemplates(si osinfo.OSRelease) bool {
	return si.ID == "rhel" || slices.Contains(si.IDLike, "rhel") || si.VersionID == "eln"
}

func LabelForISO(os *osinfo.OSRelease, arch string) string {
	switch os.ID {
	case "fedora":
		return fmt.Sprintf("Fedora-S-dvd-%s-%s", arch, os.VersionID)
	case "centos":
		labelTemplate := "CentOS-Stream-%s-BaseOS-%s"
		if os.VersionID == "8" {
			labelTemplate = "CentOS-Stream-%s-%s-dvd"
		}
		return fmt.Sprintf(labelTemplate, os.VersionID, arch)
	case "rhel":
		version := strings.ReplaceAll(os.VersionID, ".", "-")
		return fmt.Sprintf("RHEL-%s-BaseOS-%s", version, arch)
	default:
		return fmt.Sprintf("Container-Installer-%s", arch)
	}
}
