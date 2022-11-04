package rhel9

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/image"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

func osCustomizations(
	t *imageType,
	osPackageSet rpmmd.PackageSet,
	options distro.ImageOptions,
	c *blueprint.Customizations,
) manifest.OSCustomizations {

	imageConfig := t.getDefaultImageConfig()

	osc := manifest.OSCustomizations{}

	if t.bootable || t.rpmOstree {
		osc.KernelName = c.GetKernel().Name

		var kernelOptions []string
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		osc.KernelOptionsAppend = kernelOptions
	}

	osc.ExtraBasePackages = osPackageSet.Include
	osc.ExcludeBasePackages = osPackageSet.Exclude
	osc.ExtraBaseRepos = osPackageSet.Repositories

	osc.GPGKeyFiles = imageConfig.GPGKeyFiles
	if imageConfig.ExcludeDocs != nil {
		osc.ExcludeDocs = *imageConfig.ExcludeDocs
	}

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = users.GroupsFromBP(c.GetGroups())
		osc.Users = users.UsersFromBP(c.GetUsers())
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	if imageConfig.DefaultTarget != nil {
		osc.DefaultTarget = *imageConfig.DefaultTarget
	}

	osc.Firewall = c.GetFirewall()

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		osc.Language = *language
	} else if imageConfig.Locale != nil {
		osc.Language = *imageConfig.Locale
	}
	if keyboard != nil {
		osc.Keyboard = keyboard
	} else if imageConfig.Keyboard != nil {
		osc.Keyboard = &imageConfig.Keyboard.Keymap
		if imageConfig.Keyboard.X11Keymap != nil {
			osc.X11KeymapLayouts = imageConfig.Keyboard.X11Keymap.Layouts
		}
	}

	if hostname := c.GetHostname(); hostname != nil {
		osc.Hostname = *hostname
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		osc.Timezone = *timezone
	} else if imageConfig.Timezone != nil {
		osc.Timezone = *imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		for _, server := range ntpServers {
			osc.NTPServers = append(osc.NTPServers, osbuild.ChronyConfigServer{Hostname: server})
		}
	} else if imageConfig.TimeSynchronization != nil {
		osc.NTPServers = imageConfig.TimeSynchronization.Servers
		osc.LeapSecTZ = imageConfig.TimeSynchronization.LeapsecTz
	}

	// Relabel the tree, unless the `NoSElinux` flag is explicitly set to `true`
	if imageConfig.NoSElinux == nil || imageConfig.NoSElinux != nil && !*imageConfig.NoSElinux {
		osc.SElinux = "targeted"
	}

	if oscapConfig := c.GetOpenSCAP(); oscapConfig != nil {
		if t.rpmOstree {
			panic("unexpected oscap options for ostree image type")
		}
		osc.OpenSCAPConfig = osbuild.NewOscapRemediationStageOptions(
			osbuild.OscapConfig{
				Datastream: oscapConfig.DataStream,
				ProfileID:  oscapConfig.ProfileID,
			},
		)
	}

	osc.Grub2Config = imageConfig.Grub2Config
	osc.Sysconfig = imageConfig.Sysconfig
	osc.SystemdLogind = imageConfig.SystemdLogind
	osc.CloudInit = imageConfig.CloudInit
	osc.Modprobe = imageConfig.Modprobe
	osc.DracutConf = imageConfig.DracutConf
	osc.SystemdUnit = imageConfig.SystemdUnit
	osc.Authselect = imageConfig.Authselect
	osc.SELinuxConfig = imageConfig.SELinuxConfig
	osc.Tuned = imageConfig.Tuned
	osc.Tmpfilesd = imageConfig.Tmpfilesd
	osc.PamLimitsConf = imageConfig.PamLimitsConf
	osc.Sysctld = imageConfig.Sysctld
	osc.DNFConfig = imageConfig.DNFConfig
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.RHSMConfig = imageConfig.RHSMConfig
	osc.Subscription = options.Subscription

	return osc
}

func liveImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewLiveImage()
	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], options, customizations)
	img.Environment = t.environment
	img.Workload = workload
	img.Compression = t.compression
	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}

func edgeCommitImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewOSTreeArchive(options.OSTree.ImageRef)

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], options, customizations)
	img.Environment = t.environment
	img.Workload = workload

	if options.OSTree.FetchChecksum != "" && options.OSTree.URL != "" {
		img.OSTreeParent = &ostree.CommitSpec{
			Checksum:   options.OSTree.FetchChecksum,
			URL:        options.OSTree.URL,
			ContentURL: options.OSTree.ContentURL,
		}
	}

	img.OSVersion = t.arch.distro.osVersion

	img.Filename = t.Filename()

	return img, nil
}

func edgeContainerImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewOSTreeContainer(options.OSTree.ImageRef)

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], options, customizations)
	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = t.environment
	img.Workload = workload

	if options.OSTree.FetchChecksum != "" && options.OSTree.URL != "" {
		img.OSTreeParent = &ostree.CommitSpec{
			Checksum:   options.OSTree.FetchChecksum,
			URL:        options.OSTree.URL,
			ContentURL: options.OSTree.ContentURL,
		}
	}

	img.OSVersion = t.arch.distro.osVersion

	img.ExtraContainerPackages = packageSets[containerPkgsKey]

	img.Filename = t.Filename()

	return img, nil
}

func edgeInstallerImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) (image.ImageKind, error) {

	d := t.arch.distro

	commit := ostree.CommitSpec{
		Ref:        options.OSTree.ImageRef,
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
		Checksum:   options.OSTree.FetchChecksum,
	}
	img := image.NewOSTreeInstaller(commit)

	img.Platform = t.platform
	img.ExtraBasePackages = packageSets[installerPkgsKey]
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.ISOLabelTempl = d.isolabelTmpl
	img.Product = d.product
	img.Variant = "edge"
	img.OSName = "rhel"
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func edgeRawImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) (image.ImageKind, error) {

	commit := ostree.CommitSpec{
		Ref:        options.OSTree.ImageRef,
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
		Checksum:   options.OSTree.FetchChecksum,
	}
	img := image.NewOSTreeRawImage(commit)

	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.KernelOptionsAppend = []string{"modprobe.blacklist=vc4"}
	img.Keyboard = "us"
	img.Locale = "C.UTF-8"

	img.Platform = t.platform
	img.Workload = workload
	img.Remote = ostree.Remote{
		Name:       "rhel-edge",
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
	}
	img.OSName = "redhat"

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}
