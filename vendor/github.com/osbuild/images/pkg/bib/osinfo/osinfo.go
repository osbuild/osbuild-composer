package osinfo

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/bib/blueprintload"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
)

// XXX: use image-builder instead?
const bibPathPrefix = "usr/lib/bootc-image-builder"

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

type Info struct {
	OSRelease          OSRelease `yaml:"os_release"`
	UEFIVendor         string    `yaml:"uefi_vendor"`
	SELinuxPolicy      string    `yaml:"selinux_policy"`
	ImageCustomization *blueprint.Customizations
	KernelInfo         *KernelInfo `yaml:"kernel_info"`

	MountConfiguration *osbuild.MountConfiguration
	PartitionTable     *disk.PartitionTable
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

func uefiVendor(root string) (string, error) {
	bootupdEfiDir := path.Join(root, "usr/lib/bootupd/updates/EFI")
	l, err := os.ReadDir(bootupdEfiDir)
	if err != nil {
		return "", fmt.Errorf("cannot read bootupd EFI directory %s: %w", bootupdEfiDir, err)
	}

	// best-effort search: return the first directory that's not "BOOT"
	for _, entry := range l {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() == "BOOT" {
			continue
		}

		return entry.Name(), nil
	}

	return "", fmt.Errorf("cannot find UEFI vendor in %s", bootupdEfiDir)
}

func readSelinuxPolicy(root string) (string, error) {
	configPath := "etc/selinux/config"
	f, err := os.Open(path.Join(root, configPath))
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

func readImageCustomization(root string) (*blueprint.Customizations, error) {
	prefix := path.Join(root, bibPathPrefix)
	config, err := blueprintload.Load(path.Join(prefix, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if config == nil {
		config, err = blueprintload.Load(path.Join(prefix, "config.toml"))
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

func readDiskYaml(root string) (*diskYAML, error) {
	p := path.Join(root, bibPathPrefix, "disk.yaml")
	var disk diskYAML
	f, err := os.Open(p)
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

func readKernelInfo(root string) (*KernelInfo, error) {
	modulesDir := path.Join(root, "usr/lib/modules")
	entries, err := os.ReadDir(modulesDir)
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
		_, err := os.Stat(kernelPath)
		if err == nil {

			abootPath := path.Join(kernelDir, "aboot.img")
			_, err := os.Stat(abootPath)
			hasAbootImg := err == nil
			return &KernelInfo{
				Version:     e.Name(),
				HasAbootImg: hasAbootImg,
			}, nil
		}
	}

	return nil, fmt.Errorf("no valid kernel modules directory")
}

func Load(root string) (*Info, error) {
	osrelease, err := distro.ReadOSReleaseFromTree(root)
	if err != nil {
		return nil, err
	}
	if err := validateOSRelease(osrelease); err != nil {
		return nil, err
	}

	vendor, err := uefiVendor(root)
	if err != nil {
		logrus.Debugf("cannot read UEFI vendor: %v, setting it to none", err)
	}

	customization, err := readImageCustomization(root)
	if err != nil {
		return nil, err
	}

	diskYaml, err := readDiskYaml(root)
	if err != nil {
		return nil, err
	}
	var mc *osbuild.MountConfiguration
	var pt *disk.PartitionTable
	if diskYaml != nil {
		mc = diskYaml.MountConfiguration
		pt = diskYaml.PartitionTable
	}

	kernelInfo, err := readKernelInfo(root)
	if err != nil {
		logrus.Debugf("cannot read kernel info: %v", err)
	}

	selinuxPolicy, err := readSelinuxPolicy(root)
	if err != nil {
		logrus.Debugf("cannot read selinux policy: %v, setting it to none", err)
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
	}, nil
}
