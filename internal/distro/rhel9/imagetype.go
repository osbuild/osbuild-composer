package rhel9

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/oscap"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// main/common os image package set name
	osPkgsKey = "packages"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"
)

type pipelinesFunc func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, containers []container.Spec, rng *rand.Rand) ([]osbuild.Pipeline, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
	arch               *architecture
	name               string
	nameAliases        []string
	filename           string
	mimeType           string
	packageSets        map[string]packageSetFunc
	packageSetChains   map[string][]string
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	pipelines          pipelinesFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: edge/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// If set to a value, it is preferred over the architecture value
	bootType distro.BootType
	// List of valid arches for the image type
	basePartitionTables distro.BasePartitionTableMap
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.Arch().Name())
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%common.MebiByte != 0 {
		size = (size/common.MebiByte + 1) * common.MebiByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) getPackages(name string) rpmmd.PackageSet {
	getter := t.packageSets[name]
	if getter == nil {
		return rpmmd.PackageSet{}
	}

	return getter(t)
}

func (t *imageType) PackageSets(bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	mergedSets := make(map[string]rpmmd.PackageSet)

	imageSets := t.packageSets

	for name := range imageSets {
		mergedSets[name] = t.getPackages(name)
	}

	if _, hasPackages := imageSets[osPkgsKey]; !hasPackages {
		// should this be possible??
		mergedSets[osPkgsKey] = rpmmd.PackageSet{}
	}

	// every image type must define a 'build' package set
	if _, hasBuild := imageSets[buildPkgsKey]; !hasBuild {
		panic(fmt.Sprintf("'%s' image type has no '%s' package set defined", t.name, buildPkgsKey))
	}

	// blueprint packages
	bpPackages := bp.GetPackages()
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		bpPackages = append(bpPackages, "chrony")
	}

	// if we have file system customization that will need to a new mount point
	// the layout is converted to LVM so we need to corresponding packages
	if !t.rpmOstree {
		archName := t.arch.Name()
		pt := t.basePartitionTables[archName]
		haveNewMountpoint := false

		if fs := bp.Customizations.GetFilesystems(); fs != nil {
			for i := 0; !haveNewMountpoint && i < len(fs); i++ {
				haveNewMountpoint = !pt.ContainsMountpoint(fs[i].Mountpoint)
			}
		}

		if haveNewMountpoint {
			bpPackages = append(bpPackages, "lvm2")
		}
	}

	// if we are embedding containers we need to have `skopeo` in the build root
	if len(bp.Containers) > 0 {

		extraPkgs := rpmmd.PackageSet{Include: []string{"skopeo"}}

		if t.rpmOstree {
			// for OSTree based images we need to configure the containers-storage.conf(5)
			// via the org.osbuild.containers.storage.conf stage, which needs python3-toml
			extraPkgs = extraPkgs.Append(rpmmd.PackageSet{Include: []string{"python3-toml"}})
		}

		mergedSets[buildPkgsKey] = mergedSets[buildPkgsKey].Append(extraPkgs)
	}

	// if oscap customizations are enabled we need to add
	// `openscap-scanner` & `scap-security-guide` packages
	// to build root
	if bp.Customizations.GetOpenSCAP() != nil {
		bpPackages = append(bpPackages, "openscap-scanner", "scap-security-guide")
	}

	// depsolve bp packages separately
	// bp packages aren't restricted by exclude lists
	mergedSets[blueprintPkgsKey] = rpmmd.PackageSet{Include: bpPackages}
	kernel := bp.Customizations.GetKernel().Name

	// add bp kernel to main OS package set to avoid duplicate kernels
	mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(rpmmd.PackageSet{Include: []string{kernel}})

	return distro.MakePackageSetChains(t, mergedSets, repos)
}

func (t *imageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *imageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) PackageSetsChains() map[string][]string {
	return t.packageSetChains
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

// getBootType returns the BootType which should be used for this particular
// combination of architecture and image type.
func (t *imageType) getBootType() distro.BootType {
	bootType := t.arch.bootType
	if t.bootType != distro.UnsetBootType {
		bootType = t.bootType
	}
	return bootType
}

func (t *imageType) supportsUEFI() bool {
	bootType := t.getBootType()
	if bootType == distro.HybridBootType || bootType == distro.UEFIBootType {
		return true
	}
	return false
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.basePartitionTables[archName]

	if !exists {
		return nil, fmt.Errorf("unknown arch: " + archName)
	}

	imageSize := t.Size(options.Size)

	lvmify := !t.rpmOstree

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, lvmify, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.getDefaultImageConfig())

}

func (t *imageType) PartitionType() string {
	archName := t.arch.Name()
	basePartitionTable, exists := t.basePartitionTables[archName]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	containers []container.Spec,
	seed int64) (distro.Manifest, error) {

	if err := t.checkOptions(customizations, options, containers); err != nil {
		return distro.Manifest{}, err
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	pipelines, err := t.pipelines(t, customizations, options, repos, packageSpecSets, containers, rng)
	if err != nil {
		return distro.Manifest{}, err
	}

	// flatten spec sets for sources
	allPackageSpecs := make([]rpmmd.PackageSpec, 0)
	for _, specs := range packageSpecSets {
		allPackageSpecs = append(allPackageSpecs, specs...)
	}

	// handle OSTree commit inputs
	var commits []ostree.CommitSpec
	if options.OSTree.FetchChecksum != "" && options.OSTree.URL != "" {
		commit := ostree.CommitSpec{Checksum: options.OSTree.FetchChecksum, URL: options.OSTree.URL, ContentURL: options.OSTree.ContentURL}
		if options.OSTree.RHSM {
			commit.Secrets = "org.osbuild.rhsm.consumer"
		}
		commits = []ostree.CommitSpec{commit}
	}

	// handle inline sources
	inlineData := []string{}

	// FDO root certs, if any, are transmitted via an inline source
	if fdo := customizations.GetFDO(); fdo != nil && fdo.DiunPubKeyRootCerts != "" {
		inlineData = append(inlineData, fdo.DiunPubKeyRootCerts)
	}

	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   osbuild.GenSources(allPackageSpecs, commits, inlineData, containers),
		},
	)
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions, containers []container.Spec) error {

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(containers) > 0 && t.rpmOstree && (t.name != "edge-commit" && t.name != "edge-container") {
		return fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if t.bootISO && t.rpmOstree {
		// check the checksum instead of the URL, because the URL should have been used to resolve the checksum and we need both
		if options.OSTree.FetchChecksum == "" {
			return fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}

		if t.name == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.name)
			}

			// FDO is optional, but when specified has some restrictions
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to when using FDO", t.name)
				}
				var diunSet int
				if customizations.GetFDO().DiunPubKeyHash != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyInsecure != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyRootCerts != "" {
					diunSet++
				}
				if diunSet != 1 {
					return fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to when using FDO", t.name)
				}
			}
		} else if t.name == "edge-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	// check the checksum instead of the URL, because the URL should have been used to resolve the checksum and we need both
	if t.name == "edge-raw-image" && options.OSTree.FetchChecksum == "" {
		return fmt.Errorf("edge raw images require specifying a URL from which to retrieve the OSTree commit")
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	err := disk.CheckMountpoints(mountpoints, disk.MountpointPolicies)
	if err != nil {
		return err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.arch.distro.osVersion == "9.0" {
			return fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported os version: %s", t.arch.distro.osVersion))
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList) {
			return fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
		}
		if osc.DataStream == "" {
			return fmt.Errorf("OpenSCAP datastream cannot be empty")
		}
		if osc.ProfileID == "" {
			return fmt.Errorf("OpenSCAP profile cannot be empty")
		}
	}

	return nil
}
