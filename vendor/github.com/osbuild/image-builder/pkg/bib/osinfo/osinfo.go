package osinfo

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/pkg/bib/blueprintload"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/olog"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

var searchPaths = [2]string{
	"usr/lib/image-builder/bootc",
	"usr/lib/bootc-image-builder",
}

func resolvePrefix(fsys fs.FS) string {
	_, err := fs.Stat(fsys, searchPaths[0])
	if err == nil {
		return searchPaths[0]
	}
	return searchPaths[1]
}

type OSRelease struct {
	PlatformID string
	ID         string
	VersionID  string
	Name       string
	VariantID  string
	IDLike     []string
}

type KernelInfo struct {
	Version     string
	HasAbootImg bool
}

type ISOInfoGrub2Entry struct {
	Name   string
	Linux  string
	Initrd string
}

type ISOInfo struct {
	Label      string
	KernelArgs []string
	Grub2      struct {
		Default *int
		Timeout *int
		Entries []ISOInfoGrub2Entry
	}
}

type Info struct {
	OSRelease          OSRelease `yaml:"os_release"`
	UEFIVendor         string    `yaml:"uefi_vendor"`
	SELinuxPolicy      string    `yaml:"selinux_policy"`
	ImageCustomization *blueprint.Customizations
	KernelInfo         *KernelInfo `yaml:"kernel_info"`
	InitrdModules      []string    `yaml:"initrd_modules"`
	ISOInfo            ISOInfo     `yaml:"iso_info"`

	MountConfiguration *osbuild.MountConfiguration
	PartitionTable     *disk.PartitionTable
}

// HasModules returns true if all of the requested modules are in the InitrdModules list
// It returns true if the requested module list is empty
// It returns an error if the InitrdModules list is empty, and at least one
// module has been requested.
func (info *Info) HasModules(modules []string) (bool, error) {
	if len(modules) == 0 {
		return true, nil
	}
	if len(info.InitrdModules) == 0 {
		return false, fmt.Errorf("The initrd module list is empty")
	}
	for _, m := range modules {
		if !slices.Contains(info.InitrdModules, m) {
			return false, nil
		}
	}
	return true, nil
}

// GetDiskYamlRootFs returns the root filesystem type from the partition table
// defined in disk.yaml, or an empty string if not defined.
func (info *Info) GetDiskYamlRootFs() string {
	if info == nil || info.PartitionTable == nil {
		return ""
	}
	rootMountable := info.PartitionTable.FindMountable("/")
	if rootMountable != nil {
		return rootMountable.GetFSType()
	}
	return ""
}

func validateOSRelease(osrelease map[string]string) error {
	// VARIANT_ID, PLATFORM_ID are optional
	for _, key := range []string{"ID", "VERSION_ID", "NAME"} {
		if _, ok := osrelease[key]; !ok {
			return fmt.Errorf("missing %s in os-release", key)
		}
	}
	return nil
}

func uefiVendor(fsys fs.FS) (string, error) {
	var searchPath = []string{
		"usr/lib/bootupd/updates/EFI/*",
		"usr/lib/efi/shim/*/EFI/*",
	}
	for _, baseDir := range searchPath {
		dents, err := fs.Glob(fsys, baseDir)
		if err != nil {
			return "", err
		}
		// best-effort search: return the first directory that's not "BOOT"
		for _, p := range dents {
			entry, err := fs.Stat(fsys, p)
			if err != nil {
				return "", err
			}
			if !entry.IsDir() {
				continue
			}
			if entry.Name() == "BOOT" {
				continue
			}
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("cannot find UEFI vendor in %s", searchPath)
}

func readSelinuxPolicy(fsys fs.FS) (string, error) {
	configPath := "etc/selinux/config"
	f, err := fsys.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("cannot read selinux config %s: %w", configPath, err)
	}
	// nolint:errcheck
	defer f.Close()

	policy := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return "", errors.New("selinux config: invalid input")
		}
		key := strings.TrimSpace(parts[0])
		if key == "SELINUXTYPE" {
			policy = strings.TrimSpace(parts[1])
		}
	}

	return policy, nil
}

func readImageCustomization(fsys fs.FS) (*blueprint.Customizations, error) {
	// note that we only look at the 'old' search path here, we do want to
	// look in the new path as well but i'd like to only support the actual
	// blueprint format there instead of buildconfig as well
	prefix := searchPaths[1]

	config, err := blueprintload.LoadFS(fsys, path.Join(prefix, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if config == nil {
		config, err = blueprintload.LoadFS(fsys, path.Join(prefix, "config.toml"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	// no config found in either toml/json
	if config == nil {
		return nil, nil
	}

	return config.Customizations, nil
}

type diskYAML struct {
	MountConfiguration *osbuild.MountConfiguration `json:"mount_configuration" yaml:"mount_configuration"`
	PartitionTable     *disk.PartitionTable        `json:"partition_table" yaml:"partition_table"`
}

func readDiskYaml(fsys fs.FS, prefix string) (*diskYAML, error) {
	var disk diskYAML
	p := path.Join(prefix, "disk.yaml")
	f, err := fsys.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot load disk definitions from %q: %w", p, err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&disk); err != nil {
		return nil, fmt.Errorf("cannot parse disk definitions from %q: %w", p, err)
	}

	return &disk, nil
}

type isoYAML struct {
	Label      string   `json:"label" yaml:"label"`
	KernelArgs []string `json:"kernel_args" yaml:"kernel_args"`
	Grub2      struct {
		Default *int `json:"default" yaml:"default"`
		Timeout *int `json:"timeout" yaml:"timeout"`
		Entries []struct {
			Name   string `json:"name" yaml:"name"`
			Linux  string `json:"linux" yaml:"linux"`
			Initrd string `json:"initrd" yaml:"initrd"`
		} `json:"entries" yaml:"entries"`
	} `json:"grub2" yaml:"grub2"`
}

func readISOYaml(fsys fs.FS, prefix string) (*isoYAML, error) {
	var iso isoYAML
	p := path.Join(prefix, "iso.yaml")
	f, err := fsys.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot load iso definitions from %q: %w", p, err)
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&iso); err != nil {
		return nil, fmt.Errorf("cannot parse iso definitions from %q: %w", p, err)
	}

	return &iso, nil
}

func readKernelInfo(fsys fs.FS) (*KernelInfo, error) {
	modulesDir := "usr/lib/modules"
	entries, err := fs.ReadDir(fsys, modulesDir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// A kernel dir is valid if there is a vmlinuz in it.
		// bootc checks that there is only one such dir, so we
		// pick the first here
		kernelDir := path.Join(modulesDir, e.Name())
		kernelPath := path.Join(kernelDir, "vmlinuz")
		_, err := fs.Stat(fsys, kernelPath)
		if err == nil {

			abootPath := path.Join(kernelDir, "aboot.img")
			_, err := fs.Stat(fsys, abootPath)
			hasAbootImg := err == nil
			return &KernelInfo{
				Version:     e.Name(),
				HasAbootImg: hasAbootImg,
			}, nil
		}
	}

	return nil, fmt.Errorf("no valid kernel modules directory")
}

func Load(fsys fs.FS) (*Info, error) {
	osrelease, err := distro.ReadOSReleaseFromFS(fsys)
	if err != nil {
		return nil, err
	}
	if err := validateOSRelease(osrelease); err != nil {
		return nil, err
	}

	if _, err := fs.Stat(fsys, searchPaths[1]); err == nil {
		olog.Printf("WARNING: /%s found in container, this path is deprecated, please use /%s", searchPaths[1], searchPaths[0])
	}

	vendor, err := uefiVendor(fsys)
	if err != nil {
		olog.Printf("cannot read UEFI vendor: %v, setting it to none", err)
	}

	prefix := resolvePrefix(fsys)

	customization, err := readImageCustomization(fsys)
	if err != nil {
		return nil, err
	}

	diskYaml, err := readDiskYaml(fsys, prefix)
	if err != nil {
		return nil, err
	}
	var mc *osbuild.MountConfiguration
	var pt *disk.PartitionTable
	if diskYaml != nil {
		mc = diskYaml.MountConfiguration
		pt = diskYaml.PartitionTable
	}

	isoYaml, err := readISOYaml(fsys, prefix)
	if err != nil {
		return nil, err
	}

	isoInfo := ISOInfo{}

	if isoYaml != nil {
		isoInfo.Label = isoYaml.Label
		isoInfo.KernelArgs = isoYaml.KernelArgs
		isoInfo.Grub2.Default = isoYaml.Grub2.Default
		isoInfo.Grub2.Timeout = isoYaml.Grub2.Timeout

		for _, isoEntry := range isoYaml.Grub2.Entries {
			isoInfo.Grub2.Entries = append(isoInfo.Grub2.Entries, ISOInfoGrub2Entry{
				Name:   isoEntry.Name,
				Linux:  isoEntry.Linux,
				Initrd: isoEntry.Initrd,
			})
		}
	}

	kernelInfo, err := readKernelInfo(fsys)
	if err != nil {
		olog.Printf("cannot read kernel info: %v", err)
	}

	selinuxPolicy, err := readSelinuxPolicy(fsys)
	if err != nil {
		olog.Printf("cannot read selinux policy: %v, setting it to none", err)
	}

	var idLike []string
	if osrelease["ID_LIKE"] != "" {
		idLike = strings.Split(osrelease["ID_LIKE"], " ")
	}

	return &Info{
		OSRelease: OSRelease{
			ID:         osrelease["ID"],
			VersionID:  osrelease["VERSION_ID"],
			Name:       osrelease["NAME"],
			PlatformID: osrelease["PLATFORM_ID"],
			VariantID:  osrelease["VARIANT_ID"],
			IDLike:     idLike,
		},

		UEFIVendor:         vendor,
		SELinuxPolicy:      selinuxPolicy,
		ImageCustomization: customization,
		KernelInfo:         kernelInfo,
		MountConfiguration: mc,
		PartitionTable:     pt,
		ISOInfo:            isoInfo,
	}, nil
}
