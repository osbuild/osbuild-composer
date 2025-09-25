package bootc

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/osbuild/blueprint/pkg/blueprint"

	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/pathpolicy"
	"github.com/osbuild/images/pkg/platform"
)

const (
	DEFAULT_SIZE = uint64(10 * GibiByte)

	// As a baseline heuristic we double the size of
	// the input container to support in-place updates.
	// This is planned to be more configurable in the
	// future.
	containerSizeToDiskSizeMultiplier = 2
)

var (
	// The mountpoint policy for bootc images is more restrictive than the
	// ostree mountpoint policy defined in osbuild/images. It only allows /
	// (for sizing the root partition) and custom mountpoints under /var but
	// not /var itself.

	// Since our policy library doesn't support denying a path while allowing
	// its subpaths (only the opposite), we augment the standard policy check
	// with a simple search through the custom mountpoints to deny /var
	// specifically.
	mountpointPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints (but no subdirs) to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},

		// /var is not allowed, but we need to allow any subdirectories that
		// are not denied below, so we allow it initially and then check it
		// separately (in checkMountpoints())
		"/var": {Deny: false},

		// /var subdir denials
		"/var/home":     {Deny: true},
		"/var/lock":     {Deny: true}, // symlink to ../run/lock which is on tmpfs
		"/var/mail":     {Deny: true}, // symlink to spool/mail
		"/var/mnt":      {Deny: true},
		"/var/roothome": {Deny: true},
		"/var/run":      {Deny: true}, // symlink to ../run which is on tmpfs
		"/var/srv":      {Deny: true},
		"/var/usrlocal": {Deny: true},
	})

	mountpointMinimalPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},
	})
)

func (t *BootcImageType) basePartitionTable() (*disk.PartitionTable, error) {
	// base partition table can come from the container
	if t.arch.distro.sourceInfo != nil && t.arch.distro.sourceInfo.PartitionTable != nil {
		return t.arch.distro.sourceInfo.PartitionTable, nil
	}
	// get it from the build-in fallback partition tables
	if pt, ok := partitionTables[t.arch.Name()]; ok {
		return &pt, nil
	}
	return nil, fmt.Errorf("cannot find a base partition table for %q", t.Name())
}

func (t *BootcImageType) genPartitionTable(customizations *blueprint.Customizations, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	fsCust := customizations.GetFilesystems()
	diskCust, err := customizations.GetPartitioning()
	if err != nil {
		return nil, fmt.Errorf("error reading disk customizations: %w", err)
	}
	basept, err := t.basePartitionTable()
	if err != nil {
		return nil, err
	}

	// Embedded disk customization applies if there was no local customization
	if fsCust == nil && diskCust == nil && t.arch.distro.sourceInfo != nil && t.arch.distro.sourceInfo.ImageCustomization != nil {
		imageCustomizations := t.arch.distro.sourceInfo.ImageCustomization

		fsCust = imageCustomizations.GetFilesystems()
		diskCust, err = imageCustomizations.GetPartitioning()
		if err != nil {
			return nil, fmt.Errorf("error reading disk customizations: %w", err)
		}
	}

	var partitionTable *disk.PartitionTable
	switch {
	// XXX: move into images library
	case fsCust != nil && diskCust != nil:
		return nil, fmt.Errorf("cannot combine disk and filesystem customizations")
	case diskCust != nil:
		partitionTable, err = t.genPartitionTableDiskCust(basept, diskCust, rootfsMinSize, rng)
		if err != nil {
			return nil, err
		}
	default:
		partitionTable, err = t.genPartitionTableFsCust(basept, fsCust, rootfsMinSize, rng)
		if err != nil {
			return nil, err
		}
	}

	// Ensure ext4 rootfs has fs-verity enabled
	rootfs := partitionTable.FindMountable("/")
	if rootfs != nil {
		switch elem := rootfs.(type) {
		case *disk.Filesystem:
			if elem.Type == "ext4" {
				elem.MkfsOptions.Verity = true
			}
		}
	}

	return partitionTable, nil
}

func (t *BootcImageType) genPartitionTableDiskCust(basept *disk.PartitionTable, diskCust *blueprint.DiskCustomization, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	if err := diskCust.ValidateLayoutConstraints(); err != nil {
		return nil, fmt.Errorf("cannot use disk customization: %w", err)
	}

	diskCust.MinSize = max(diskCust.MinSize, rootfsMinSize)

	if basept == nil {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", t.arch.Name())
	}
	defaultFSType, err := disk.NewFSType(t.arch.distro.defaultFs)
	if err != nil {
		return nil, err
	}
	requiredMinSizes, err := calcRequiredDirectorySizes(diskCust, rootfsMinSize)
	if err != nil {
		return nil, err
	}
	partOptions := &disk.CustomPartitionTableOptions{
		PartitionTableType: basept.Type,
		// XXX: not setting/defaults will fail to boot with btrfs/lvm
		BootMode:         platform.BOOT_HYBRID,
		DefaultFSType:    defaultFSType,
		RequiredMinSizes: requiredMinSizes,
		Architecture:     t.arch.arch,
	}
	return disk.NewCustomPartitionTable(diskCust, partOptions, rng)
}

func (t *BootcImageType) genPartitionTableFsCust(basept *disk.PartitionTable, fsCust []blueprint.FilesystemCustomization, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	if basept == nil {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", t.arch.Name())
	}

	partitioningMode := partition.RawPartitioningMode
	if t.arch.distro.defaultFs == "btrfs" {
		partitioningMode = partition.BtrfsPartitioningMode
	}
	if err := checkFilesystemCustomizations(fsCust, partitioningMode); err != nil {
		return nil, err
	}
	fsCustomizations := updateFilesystemSizes(fsCust, rootfsMinSize)

	pt, err := disk.NewPartitionTable(basept, fsCustomizations, DEFAULT_SIZE, partitioningMode, t.arch.arch, nil, t.arch.distro.defaultFs, rng)
	if err != nil {
		return nil, err
	}

	if err := setFSTypes(pt, t.arch.distro.defaultFs); err != nil {
		return nil, fmt.Errorf("error setting root filesystem type: %w", err)
	}
	return pt, nil
}

func checkMountpoints(filesystems []blueprint.FilesystemCustomization, policy *pathpolicy.PathPolicies) error {
	errs := []error{}
	for _, fs := range filesystems {
		if err := policy.Check(fs.Mountpoint); err != nil {
			errs = append(errs, err)
		}
		if fs.Mountpoint == "/var" {
			// this error message is consistent with the errors returned by policy.Check()
			// TODO: remove trailing space inside the quoted path when the function is fixed in osbuild/images.
			errs = append(errs, fmt.Errorf(`path "/var" is not allowed`))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("the following errors occurred while validating custom mountpoints:\n%w", errors.Join(errs...))
	}
	return nil
}

// calcRequiredDirectorySizes will calculate the minimum sizes for /
// for disk customizations. We need this because with advanced partitioning
// we never grow the rootfs to the size of the disk (unlike the tranditional
// filesystem customizations).
//
// So we need to go over the customizations and ensure the min-size for "/"
// is at least rootfsMinSize.
//
// Note that a custom "/usr" is not supported in image mode so splitting
// rootfsMinSize between / and /usr is not a concern.
func calcRequiredDirectorySizes(distCust *blueprint.DiskCustomization, rootfsMinSize uint64) (map[string]uint64, error) {
	// XXX: this has *way* too much low-level knowledge about the
	// inner workings of blueprint.DiskCustomizations plus when
	// a new type it needs to get added here too, think about
	// moving into "images" instead (at least partly)
	mounts := map[string]uint64{}
	for _, part := range distCust.Partitions {
		switch part.Type {
		case "", "plain":
			mounts[part.Mountpoint] = part.MinSize
		case "lvm":
			for _, lv := range part.LogicalVolumes {
				mounts[lv.Mountpoint] = part.MinSize
			}
		case "btrfs":
			for _, subvol := range part.Subvolumes {
				mounts[subvol.Mountpoint] = part.MinSize
			}
		default:
			return nil, fmt.Errorf("unknown disk customization type %q", part.Type)
		}
	}
	// ensure rootfsMinSize is respected
	return map[string]uint64{
		"/": max(rootfsMinSize, mounts["/"]),
	}, nil
}

func checkFilesystemCustomizations(fsCustomizations []blueprint.FilesystemCustomization, ptmode partition.PartitioningMode) error {
	var policy *pathpolicy.PathPolicies
	switch ptmode {
	case partition.BtrfsPartitioningMode:
		// btrfs subvolumes are not supported at build time yet, so we only
		// allow / and /boot to be customized when building a btrfs disk (the
		// minimal policy)
		policy = mountpointMinimalPolicy
	default:
		policy = mountpointPolicy
	}
	if err := checkMountpoints(fsCustomizations, policy); err != nil {
		return err
	}
	return nil
}

// updateFilesystemSizes updates the size of the root filesystem customization
// based on the minRootSize. The new min size whichever is larger between the
// existing size and the minRootSize. If the root filesystem is not already
// configured, a new customization is added.
func updateFilesystemSizes(fsCustomizations []blueprint.FilesystemCustomization, minRootSize uint64) []blueprint.FilesystemCustomization {
	updated := make([]blueprint.FilesystemCustomization, len(fsCustomizations), len(fsCustomizations)+1)
	hasRoot := false
	for idx, fsc := range fsCustomizations {
		updated[idx] = fsc
		if updated[idx].Mountpoint == "/" {
			updated[idx].MinSize = max(updated[idx].MinSize, minRootSize)
			hasRoot = true
		}
	}

	if !hasRoot {
		// no root customization found: add it
		updated = append(updated, blueprint.FilesystemCustomization{Mountpoint: "/", MinSize: minRootSize})
	}
	return updated
}

// setFSTypes sets the filesystem types for all mountable entities to match the
// selected rootfs type.
// If rootfs is 'btrfs', the function will keep '/boot' to its default.
func setFSTypes(pt *disk.PartitionTable, rootfs string) error {
	if rootfs == "" {
		return fmt.Errorf("root filesystem type is empty")
	}

	return pt.ForEachMountable(func(mnt disk.Mountable, _ []disk.Entity) error {
		switch mnt.GetMountpoint() {
		case "/boot/efi":
			// never change the efi partition's type
			return nil
		case "/boot":
			// change only if we're not doing btrfs
			if rootfs == "btrfs" {
				return nil
			}
			fallthrough
		default:
			switch elem := mnt.(type) {
			case *disk.Filesystem:
				elem.Type = rootfs
			case *disk.BtrfsSubvolume:
				// nothing to do
			default:
				return fmt.Errorf("the mountable disk entity for %q of the base partition table is not an ordinary filesystem but %T", mnt.GetMountpoint(), mnt)
			}
			return nil
		}
	})
}
