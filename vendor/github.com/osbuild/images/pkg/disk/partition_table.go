package disk

import (
	"fmt"
	"math/rand"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/platform"
)

type PartitionTable struct {
	// Size of the disk (in bytes).
	Size uint64 `json:"size,omitempty" yaml:"size,omitempty"`
	// Unique identifier of the partition table (GPT only).
	UUID string `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	// Partition table type, e.g. dos, gpt.
	Type       PartitionTableType `json:"type" yaml:"type"`
	Partitions []Partition        `json:"partitions" yaml:"partitions"`

	// Sector size in bytes
	SectorSize uint64 `json:"sector_size,omitempty" yaml:"sector_size,omitempty"`
	// Extra space at the end of the partition table (sectors)
	ExtraPadding uint64 `json:"extra_padding,omitempty" yaml:"extra_padding,omitempty"`
	// Starting offset of the first partition in the table (Mb)
	StartOffset uint64 `json:"start_offset,omitempty" yaml:"start_offset,omitempty"`
}

type PartitioningMode string

const (
	// AutoLVMPartitioningMode creates a LVM layout if the filesystem
	// contains a mountpoint that's not defined in the base partition table
	// of the specified image type. In the other case, a raw layout is used.
	AutoLVMPartitioningMode PartitioningMode = "auto-lvm"

	// LVMPartitioningMode always creates an LVM layout.
	LVMPartitioningMode PartitioningMode = "lvm"

	// RawPartitioningMode always creates a raw layout.
	RawPartitioningMode PartitioningMode = "raw"

	// BtrfsPartitioningMode creates a btrfs layout.
	BtrfsPartitioningMode PartitioningMode = "btrfs"

	// DefaultPartitioningMode is AutoLVMPartitioningMode and is the empty state
	DefaultPartitioningMode PartitioningMode = ""
)

// NewPartitionTable takes an existing base partition table and some parameters
// and returns a new version of the base table modified to satisfy the
// parameters.
//
// Mountpoints: New filesystems and minimum partition sizes are defined in
// mountpoints. By default, if new mountpoints are created, a partition table is
// automatically converted to LVM (see Partitioning modes below).
//
// Image size: The minimum size of the partition table, which in turn will be
// the size of the disk image. The final size of the image will either be the
// value of the imageSize argument or the sum of all partitions and their
// associated metadata, whichever is larger.
//
// Partitioning modes: The mode controls how the partition table is modified.
//
//   - Raw will not convert any partition to LVM or Btrfs.
//   - LVM will convert the partition that contains the root mountpoint '/' to an
//
// LVM Volume Group and create a root Logical Volume. Any extra mountpoints,
// except /boot, will be added to the Volume Group as new Logical Volumes.
//
//   - Btrfs will convert the partition that contains the root mountpoint '/' to
//     a Btrfs volume and create a root subvolume. Any extra mountpoints, except
//     /boot, will be added to the Btrfs volume as new Btrfs subvolumes.
//   - AutoLVM is the default mode and will convert a raw partition table to an
//     LVM-based one if and only if new mountpoints are added.
//
// Directory sizes: The requiredSizes argument defines a map of minimum sizes
// for specific directories. These indirectly control the minimum sizes of
// partitions. A directory with a required size will set the minimum size of
// the partition with the mountpoint that contains the directory. Additional
// directory requirements are additive, meaning the minimum size for a
// mountpoint's partition is the sum of all the required directory sizes it
// will contain. By default, if no requiredSizes are provided, the new
// partition table will require at least 1 GiB for '/' and 2 GiB for '/usr'. In
// most cases, this translates to a requirement of 3 GiB for the root
// partition, Logical Volume, or Btrfs subvolume.
//
// # General principles:
//
// Desired sizes for partitions, partition tables, volumes, directories, etc,
// are always treated as minimum sizes. This means that very often the full
// disk image size is larger than the size of the sum of the partitions due to
// metadata. The function considers that the size of volumes have higher
// priority than the size of the disk.
//
// The partition or volume container that contains '/' is always last in the
// partition table layout.
//
// In the case of raw partitioning (no LVM and no Btrfs), the partition
// containing the root filesystem is grown to fill any left over space on the
// partition table. Logical Volumes are not grown to fill the space in the
// Volume Group since they are trivial to grow on a live system.
func NewPartitionTable(basePT *PartitionTable, mountpoints []blueprint.FilesystemCustomization, imageSize uint64, mode PartitioningMode, architecture arch.Arch, requiredSizes map[string]uint64, rng *rand.Rand) (*PartitionTable, error) {
	newPT := basePT.Clone().(*PartitionTable)

	if basePT.features().LVM && (mode == RawPartitioningMode || mode == BtrfsPartitioningMode) {
		return nil, fmt.Errorf("%s partitioning mode set for a base partition table with LVM, this is unsupported", mode)
	}

	// first pass: enlarge existing mountpoints and collect new ones
	newMountpoints, _ := newPT.applyCustomization(mountpoints, false)

	var ensureLVM, ensureBtrfs bool
	switch mode {
	case LVMPartitioningMode:
		ensureLVM = true
	case RawPartitioningMode:
		ensureLVM = false
	case DefaultPartitioningMode, AutoLVMPartitioningMode:
		ensureLVM = len(newMountpoints) > 0
	case BtrfsPartitioningMode:
		ensureBtrfs = true
	default:
		return nil, fmt.Errorf("unsupported partitioning mode %q", mode)
	}
	if ensureLVM {
		err := newPT.ensureLVM()
		if err != nil {
			return nil, err
		}
	} else if ensureBtrfs {
		err := newPT.ensureBtrfs(architecture)
		if err != nil {
			return nil, err
		}
	}

	// second pass: deal with new mountpoints and newly created ones, after switching to
	// the LVM layout, if requested, which might introduce new mount points, i.e. `/boot`
	_, err := newPT.applyCustomization(newMountpoints, true)
	if err != nil {
		return nil, err
	}

	// If no separate requiredSizes are given then we use our defaults
	if requiredSizes == nil {
		requiredSizes = map[string]uint64{
			"/":    1073741824,
			"/usr": 2147483648,
		}
	}

	if len(requiredSizes) != 0 {
		newPT.EnsureDirectorySizes(requiredSizes)
	}

	// Calculate partition table offsets and sizes
	newPT.relayout(imageSize)

	// Generate new UUIDs for filesystems and partitions
	newPT.GenerateUUIDs(rng)

	return newPT, nil
}

func (pt *PartitionTable) Clone() Entity {
	if pt == nil {
		return nil
	}

	clone := &PartitionTable{
		Size:         pt.Size,
		UUID:         pt.UUID,
		Type:         pt.Type,
		Partitions:   make([]Partition, len(pt.Partitions)),
		SectorSize:   pt.SectorSize,
		ExtraPadding: pt.ExtraPadding,
		StartOffset:  pt.StartOffset,
	}

	for idx, partition := range pt.Partitions {
		ent := partition.Clone()

		// partition.Clone() will return nil only if the partition is nil
		if ent == nil {
			panic(fmt.Sprintf("partition %d in a Partition Table is nil; this is a programming error", idx))
		}

		part, cloneOk := ent.(*Partition)
		if !cloneOk {
			panic("PartitionTable.Clone() returned an Entity that cannot be converted to *PartitionTable; this is a programming error")
		}

		clone.Partitions[idx] = *part
	}
	return clone
}

// AlignUp will round up the given size value to the default grain if not
// already aligned.
func (pt *PartitionTable) AlignUp(size uint64) uint64 {
	grain := DefaultGrainBytes
	if size%grain == 0 {
		// already aligned: return unchanged
		return size
	}
	return ((size + grain) / grain) * grain
}

// Convert the given bytes to the number of sectors.
func (pt *PartitionTable) BytesToSectors(size uint64) uint64 {
	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = DefaultSectorSize
	}
	return size / sectorSize
}

// Convert the given number of sectors to bytes.
func (pt *PartitionTable) SectorsToBytes(size uint64) uint64 {
	sectorSize := pt.SectorSize
	if sectorSize == 0 {
		sectorSize = DefaultSectorSize
	}
	return size * sectorSize
}

// Returns if the partition table contains a filesystem with the given
// mount point.
func (pt *PartitionTable) ContainsMountpoint(mountpoint string) bool {
	return len(entityPath(pt, mountpoint)) > 0
}

// Generate all needed UUIDs for all the partiton and filesystems
//
// Will not overwrite existing UUIDs and only generate UUIDs for
// partitions if the layout is GPT.
func (pt *PartitionTable) GenerateUUIDs(rng *rand.Rand) {
	setuuid := func(ent Entity, path []Entity) error {
		if ui, ok := ent.(UniqueEntity); ok {
			ui.GenUUID(rng)
		}
		return nil
	}
	_ = pt.ForEachEntity(setuuid)

	// if this is a MBR partition table, there is no need to generate
	// uuids for the partitions themselves
	if pt.Type != PT_GPT {
		return
	}

	for idx, part := range pt.Partitions {
		if part.UUID == "" {
			pt.Partitions[idx].UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
		}
	}
}

func (pt *PartitionTable) GetItemCount() uint {
	return uint(len(pt.Partitions))
}

func (pt *PartitionTable) GetChild(n uint) Entity {
	return &pt.Partitions[n]
}

func (pt *PartitionTable) GetSize() uint64 {
	return pt.Size
}

func (pt *PartitionTable) EnsureSize(s uint64) bool {
	if s > pt.Size {
		pt.Size = s
		return true
	}
	return false
}

func (pt *PartitionTable) findDirectoryEntityPath(dir string) []Entity {
	if path := entityPath(pt, dir); path != nil {
		return path
	}

	parent := filepath.Dir(dir)
	if dir == parent {
		// invalid dir or pt has no root
		return nil
	}

	// move up the directory path and check again
	return pt.findDirectoryEntityPath(parent)
}

// EnsureDirectorySizes takes a mapping of directory paths to sizes (in bytes)
// and resizes the appropriate partitions such that they are at least the size
// of the sum of their subdirectories plus their own sizes.
// The function will panic if any of the directory paths are invalid.
func (pt *PartitionTable) EnsureDirectorySizes(dirSizeMap map[string]uint64) {

	type mntSize struct {
		entPath []Entity
		newSize uint64
	}

	// add up the required size for each directory grouped by their mountpoints
	mntSizeMap := make(map[string]*mntSize)
	for dir, size := range dirSizeMap {
		entPath := pt.findDirectoryEntityPath(dir)
		if entPath == nil {
			panic(fmt.Sprintf("EnsureDirectorySizes: invalid dir path %q", dir))
		}
		mnt := entPath[0].(Mountable)
		mountpoint := mnt.GetMountpoint()
		if _, ok := mntSizeMap[mountpoint]; !ok {
			mntSizeMap[mountpoint] = &mntSize{entPath, 0}
		}
		es := mntSizeMap[mountpoint]
		es.newSize += size
	}

	// resize all the entities in the map
	for _, es := range mntSizeMap {
		resizeEntityBranch(es.entPath, es.newSize)
	}
}

func (pt *PartitionTable) CreateMountpoint(mountpoint string, size uint64) (Entity, error) {
	filesystem := Filesystem{
		Type:         "xfs",
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}

	partition := Partition{
		Size:    size,
		Payload: &filesystem,
	}

	n := len(pt.Partitions)
	var maxNo int

	if pt.Type == PT_GPT {
		switch mountpoint {
		case "/boot":
			partition.Type = XBootLDRPartitionGUID
		default:
			partition.Type = FilesystemDataGUID
		}
		maxNo = 128
	} else {
		maxNo = 4
	}

	if n == maxNo {
		return nil, fmt.Errorf("maximum number of partitions reached (%d)", maxNo)
	}

	pt.Partitions = append(pt.Partitions, partition)

	return &pt.Partitions[len(pt.Partitions)-1], nil
}

type EntityCallback func(e Entity, path []Entity) error

func forEachEntity(e Entity, path []Entity, cb EntityCallback) error {

	childPath := append(path, e)
	err := cb(e, childPath)

	if err != nil {
		return err
	}

	c, ok := e.(Container)
	if !ok {
		return nil
	}

	for idx := uint(0); idx < c.GetItemCount(); idx++ {
		child := c.GetChild(idx)
		err = forEachEntity(child, childPath, cb)
		if err != nil {
			return err
		}
	}

	return nil
}

// ForEachEntity runs the provided callback function on each entity in
// the PartitionTable.
func (pt *PartitionTable) ForEachEntity(cb EntityCallback) error {
	return forEachEntity(pt, []Entity{}, cb)
}

func (pt *PartitionTable) HeaderSize() uint64 {
	// always reserve one extra sector for the GPT header
	// this also ensure we have enough space for the MBR
	header := pt.SectorsToBytes(1)

	if pt.Type == PT_DOS {
		return header
	}

	// calculate the space we need for
	parts := uint64(len(pt.Partitions))

	// reserve a minimum of 128 partition entires
	if parts < 128 {
		parts = 128
	}

	// Assume that each partition entry is 128 bytes
	// which might not be the case if the partition
	// name exceeds 72 bytes
	header += parts * 128

	return header
}

// Apply filesystem customization to the partition table. If create is false,
// the function will only apply customizations to existing partitions and
// return a list of left-over mountpoints (i.e. mountpoints in the input that
// were not created). An error can only occur if create is set.
// Does not relayout the table, i.e. a call to relayout might be needed.
func (pt *PartitionTable) applyCustomization(mountpoints []blueprint.FilesystemCustomization, create bool) ([]blueprint.FilesystemCustomization, error) {

	newMountpoints := []blueprint.FilesystemCustomization{}

	for _, mnt := range mountpoints {
		size := clampFSSize(mnt.Mountpoint, mnt.MinSize)
		if path := entityPath(pt, mnt.Mountpoint); len(path) != 0 {
			size = alignEntityBranch(path, size)
			resizeEntityBranch(path, size)
		} else {
			if !create {
				newMountpoints = append(newMountpoints, mnt)
			} else if err := pt.createFilesystem(mnt.Mountpoint, size); err != nil {
				return nil, err
			}
		}
	}

	return newMountpoints, nil
}

// Dynamically calculate and update the start point for each of the existing
// partitions. Adjusts the overall size of image to either the supplied value
// in `size` or to the sum of all partitions if that is larger. Will grow the
// root partition if there is any empty space. Returns the updated start point.
func (pt *PartitionTable) relayout(size uint64) uint64 {
	// always reserve one extra sector for the GPT header
	header := pt.HeaderSize()
	footer := uint64(0)

	// The GPT header is also at the end of the partition table
	if pt.Type == PT_GPT {
		footer = header
	}

	start := pt.AlignUp(header)
	start += pt.StartOffset
	size = pt.AlignUp(size)

	var rootIdx = -1
	for idx := range pt.Partitions {
		partition := &pt.Partitions[idx]
		if len(entityPath(partition, "/")) != 0 {
			// keep the root partition index to handle after all the other
			// partitions have been moved and resized
			rootIdx = idx
			continue
		}
		partition.Start = start
		partition.fitTo(partition.Size)
		partition.Size = pt.AlignUp(partition.Size)
		start += partition.Size
	}

	if rootIdx < 0 {
		panic("no root filesystem found; this is a programming error")
	}

	root := &pt.Partitions[rootIdx]
	root.Start = start
	root.fitTo(root.Size)

	// add the extra padding specified in the partition table
	footer += pt.ExtraPadding

	// If the sum of all partitions is bigger then the specified size,
	// we use that instead. Grow the partition table size if needed.
	end := pt.AlignUp(root.Start + footer + root.Size)
	if end > size {
		size = end
	}

	if size > pt.Size {
		pt.Size = size
	}

	// If there is space left in the partition table, grow root
	root.Size = pt.Size - root.Start

	// Finally we shrink the last partition, i.e. the root partition,
	// to leave space for the footer, e.g. the secondary GPT header.
	root.Size -= footer

	return start
}

func (pt *PartitionTable) createFilesystem(mountpoint string, size uint64) error {
	rootPath := entityPath(pt, "/")
	if rootPath == nil {
		panic("no root mountpoint for PartitionTable")
	}

	var vc MountpointCreator
	var entity Entity
	var idx int
	for idx, entity = range rootPath {
		var ok bool
		if vc, ok = entity.(MountpointCreator); ok {
			break
		}
	}

	if vc == nil {
		panic("could not find root volume container")
	}

	newVol, err := vc.CreateMountpoint(mountpoint, 0)
	if err != nil {
		return fmt.Errorf("failed creating volume: %w", err)
	}
	vcPath := append([]Entity{newVol}, rootPath[idx:]...)
	size = alignEntityBranch(vcPath, size)
	resizeEntityBranch(vcPath, size)
	return nil
}

// entityPath stats at ent and searches for an Entity with a Mountpoint equal
// to the target. Returns a slice of all the Entities leading to the Mountable
// in reverse order. If no Entity has the target as a Mountpoint, returns nil.
// If a slice is returned, the last element is always the starting Entity ent
// and the first element is always a Mountable with a Mountpoint equal to the
// target.
func entityPath(ent Entity, target string) []Entity {
	switch e := ent.(type) {
	case Mountable:
		if target == e.GetMountpoint() {
			return []Entity{ent}
		}
	case Container:
		for idx := uint(0); idx < e.GetItemCount(); idx++ {
			child := e.GetChild(idx)
			path := entityPath(child, target)
			if path != nil {
				path = append(path, e)
				return path
			}
		}
	}
	return nil
}

type MountableCallback func(mnt Mountable, path []Entity) error

func forEachMountable(c Container, path []Entity, cb MountableCallback) error {
	for idx := uint(0); idx < c.GetItemCount(); idx++ {
		child := c.GetChild(idx)
		childPath := append(path, child)
		var err error
		switch ent := child.(type) {
		case Mountable:
			err = cb(ent, childPath)
		case Container:
			err = forEachMountable(ent, childPath, cb)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ForEachMountable runs the provided callback function on each Mountable in
// the PartitionTable.
func (pt *PartitionTable) ForEachMountable(cb MountableCallback) error {
	return forEachMountable(pt, []Entity{pt}, cb)
}

type FSTabEntityCallback func(mnt FSTabEntity, path []Entity) error

func forEachFSTabEntity(c Container, path []Entity, cb FSTabEntityCallback) error {
	for idx := uint(0); idx < c.GetItemCount(); idx++ {
		child := c.GetChild(idx)
		childPath := append(path, child)
		var err error
		switch ent := child.(type) {
		case FSTabEntity:
			err = cb(ent, childPath)
		case Container:
			err = forEachFSTabEntity(ent, childPath, cb)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ForEachFSTabEntity runs the provided callback function on each FSTabEntity
// in the PartitionTable.
func (pt *PartitionTable) ForEachFSTabEntity(cb FSTabEntityCallback) error {
	return forEachFSTabEntity(pt, []Entity{pt}, cb)
}

// FindMountable returns the Mountable entity with the given mountpoint in the
// PartitionTable. Returns nil if no Entity has the target as a Mountpoint.
func (pt *PartitionTable) FindMountable(mountpoint string) Mountable {
	path := entityPath(pt, mountpoint)

	if len(path) == 0 {
		return nil
	}
	// first path element is guaranteed to be Mountable
	return path[0].(Mountable)
}

func clampFSSize(mountpoint string, size uint64) uint64 {
	// set a minimum size of 1GB for all mountpoints
	// with the exception for '/boot' (= 500 MB)
	var minSize uint64 = 1073741824

	if mountpoint == "/boot" {
		minSize = 524288000
	}

	if minSize > size {
		return minSize
	}
	return size
}

func alignEntityBranch(path []Entity, size uint64) uint64 {
	if len(path) == 0 {
		return size
	}

	element := path[0]

	if c, ok := element.(MountpointCreator); ok {
		size = c.AlignUp(size)
	}

	return alignEntityBranch(path[1:], size)
}

// resizeEntityBranch resizes the first entity in the specified path to be at
// least the specified size and then grows every entity up the path to the
// PartitionTable accordingly.
func resizeEntityBranch(path []Entity, size uint64) {
	if len(path) == 0 {
		return
	}

	element := path[0]

	if c, ok := element.(Container); ok {
		containerSize := uint64(0)
		for idx := uint(0); idx < c.GetItemCount(); idx++ {
			if s, ok := c.GetChild(idx).(Sizeable); ok {
				containerSize += s.GetSize()
			} else {
				break
			}
		}
		// If containerSize is 0, it means it doesn't have any direct sizeable
		// children (e.g., a LUKS container with a VG child).  In that case,
		// set the containerSize to the desired size for the branch before
		// adding any metadata.
		if containerSize == 0 {
			containerSize = size
		}
		if vc, ok := element.(VolumeContainer); ok {
			containerSize += vc.MetadataSize()
		}
		if containerSize > size {
			size = containerSize
		}
	}
	if sz, ok := element.(Sizeable); ok {
		if !sz.EnsureSize(size) {
			return
		}
	}
	resizeEntityBranch(path[1:], size)
}

// GenUUID generates and sets UUIDs for all Partitions in the PartitionTable if
// the layout is GPT.
func (pt *PartitionTable) GenUUID(rng *rand.Rand) {
	if pt.UUID == "" {
		pt.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}

// ensureLVM will ensure that the root partition is on an LVM volume, i.e. if
// it currently is not, it will wrap it in one
func (pt *PartitionTable) ensureLVM() error {

	rootPath := entityPath(pt, "/")
	if rootPath == nil {
		panic("no root mountpoint for PartitionTable")
	}

	// we need a /boot partition to boot LVM, ensure one exists
	bootPath := entityPath(pt, "/boot")
	if bootPath == nil {
		_, err := pt.CreateMountpoint("/boot", 512*datasizes.MiB)

		if err != nil {
			return err
		}

		rootPath = entityPath(pt, "/")
	}

	parent := rootPath[1] // NB: entityPath has reversed order

	if _, ok := parent.(*LVMLogicalVolume); ok {
		return nil
	}
	part, ok := parent.(*Partition)
	if !ok {
		return fmt.Errorf("Unsupported parent for LVM")
	}
	filesystem := part.Payload

	vg := &LVMVolumeGroup{
		Name:        "rootvg",
		Description: "created via lvm2 and osbuild",
	}

	// create root logical volume on the new volume group with the same
	// size and filesystem as the previous root partition
	_, err := vg.CreateLogicalVolume("rootlv", part.Size, filesystem)
	if err != nil {
		panic(fmt.Sprintf("Could not create LV: %v", err))
	}

	// replace the top-level partition payload with the new volume group
	part.Payload = vg

	// reset the vg partition size - it will be grown later
	part.Size = 0

	if pt.Type == PT_GPT {
		part.Type = LVMPartitionGUID
	} else {
		part.Type = LVMPartitionDOSID
	}

	return nil
}

// ensureBtrfs will ensure that the root partition is on a btrfs subvolume, i.e. if
// it currently is not, it will wrap it in one
func (pt *PartitionTable) ensureBtrfs(architecture arch.Arch) error {

	rootPath := entityPath(pt, "/")
	if rootPath == nil {
		return fmt.Errorf("no root mountpoint for a partition table: %#v", pt)
	}

	// we need a /boot partition to boot btrfs, ensure one exists
	bootPath := entityPath(pt, "/boot")
	if bootPath == nil {
		_, err := pt.CreateMountpoint("/boot", 512*datasizes.MiB)
		if err != nil {
			return fmt.Errorf("failed to create /boot partition when ensuring btrfs: %w", err)
		}

		rootPath = entityPath(pt, "/")
	}

	parent := rootPath[1] // NB: entityPath has reversed order

	if _, ok := parent.(*Btrfs); ok {
		return nil
	}
	part, ok := parent.(*Partition)
	if !ok {
		return fmt.Errorf("unsupported parent for btrfs: %T", parent)
	}
	rootMountable, ok := rootPath[0].(Mountable)
	if !ok {
		return fmt.Errorf("root entity is not mountable: %T, this is a violation of entityPath() contract", rootPath[0])
	}

	opts, err := rootMountable.GetFSTabOptions()
	if err != nil {
		return err
	}

	btrfs := &Btrfs{
		Label: "root",
		Subvolumes: []BtrfsSubvolume{
			{
				Name:       "root",
				Mountpoint: "/",
				Compress:   DefaultBtrfsCompression,
				ReadOnly:   opts.ReadOnly(),
				Size:       part.Size,
			},
		},
	}

	// replace the top-level partition payload with a new btrfs filesystem
	part.Payload = btrfs

	// reset the btrfs partition size - it will be grown later
	part.Size = 0

	part.Type, err = getPartitionTypeIDfor(pt.Type, "root", architecture)
	if err != nil {
		return fmt.Errorf("error converting partition table to btrfs: %w", err)
	}

	return nil
}

type partitionTableFeatures struct {
	LVM   bool
	Btrfs bool
	XFS   bool
	FAT   bool
	EXT4  bool
	LUKS  bool
	Swap  bool
}

// features examines all of the PartitionTable entities and returns a struct
// with flags set for each feature used. The meaning of "feature" here is quite
// broad. Most disk Entity types are represented by a feature and the existence
// of at least one type in the partition table means the feature is
// represented. For Filesystem entities, there is a separate feature for each
// filesystem type
func (pt *PartitionTable) features() partitionTableFeatures {
	var ptFeatures partitionTableFeatures

	introspectPT := func(e Entity, path []Entity) error {
		switch ent := e.(type) {
		case *LVMVolumeGroup, *LVMLogicalVolume:
			ptFeatures.LVM = true
		case *Btrfs, *BtrfsSubvolume:
			ptFeatures.Btrfs = true
		case *Filesystem:
			switch ent.GetFSType() {
			case "vfat":
				ptFeatures.FAT = true
			case "btrfs":
				ptFeatures.Btrfs = true
			case "xfs":
				ptFeatures.XFS = true
			case "ext4":
				ptFeatures.EXT4 = true
			}
		case *Swap:
			ptFeatures.Swap = true
		case *LUKSContainer:
			ptFeatures.LUKS = true
		case *PartitionTable, *Partition:
			// nothing to do
		default:
			panic(fmt.Errorf("unknown entity type %T", e))
		}
		return nil
	}
	_ = pt.ForEachEntity(introspectPT)

	return ptFeatures
}

// GetBuildPackages returns an array of packages needed to support the features used in the PartitionTable.
func (pt *PartitionTable) GetBuildPackages() []string {
	packages := []string{}

	features := pt.features()

	if features.LVM {
		packages = append(packages, "lvm2")
	}
	if features.Btrfs {
		packages = append(packages, "btrfs-progs")
	}
	if features.XFS {
		packages = append(packages, "xfsprogs")
	}
	if features.FAT {
		packages = append(packages, "dosfstools")
	}
	if features.EXT4 {
		packages = append(packages, "e2fsprogs")
	}
	if features.LUKS {
		packages = append(packages,
			"clevis",
			"clevis-luks",
			"cryptsetup",
		)
	}

	return packages
}

// GetMountpointSize takes a mountpoint and returns the size of the entity this
// mountpoint belongs to.
func (pt *PartitionTable) GetMountpointSize(mountpoint string) (uint64, error) {
	path := entityPath(pt, mountpoint)
	if path == nil {
		return 0, fmt.Errorf("cannot find mountpoint %s", mountpoint)
	}

	for _, ent := range path {
		if sizeable, ok := ent.(Sizeable); ok {
			return sizeable.GetSize(), nil
		}
	}

	panic(fmt.Sprintf("no sizeable of the entity path for mountpoint %s, this is a programming error", mountpoint))
}

// EnsureRootFilesystem adds a root filesystem if the partition table doesn't
// already have one.
//
// When adding the root filesystem, add it to:
//
//   - The first LVM Volume Group if one exists, otherwise
//   - The first Btrfs volume if one exists, otherwise
//   - At the end of the plain partitions.
//
// For LVM and Plain, the fsType argument must be a valid filesystem type.
func EnsureRootFilesystem(pt *PartitionTable, defaultFsType FSType, architecture arch.Arch) error {
	// collect all labels and subvolume names to avoid conflicts
	subvolNames := make(map[string]bool)
	labels := make(map[string]bool)
	var foundRoot bool
	_ = pt.ForEachMountable(func(mnt Mountable, path []Entity) error {
		if mnt.GetMountpoint() == "/" {
			foundRoot = true
			return nil
		}

		labels[mnt.GetFSSpec().Label] = true
		switch mountable := mnt.(type) {
		case *BtrfsSubvolume:
			subvolNames[mountable.Name] = true
		}
		return nil
	})
	if foundRoot {
		// nothing to do
		return nil
	}

	for _, part := range pt.Partitions {
		switch payload := part.Payload.(type) {
		case *LVMVolumeGroup:
			if defaultFsType == FS_NONE {
				return fmt.Errorf("error creating root logical volume: no default filesystem type")
			}

			rootLabel, err := genUniqueString("root", labels)
			if err != nil {
				return fmt.Errorf("error creating root logical volume: %w", err)
			}
			rootfs := &Filesystem{
				Type:         defaultFsType.String(),
				Label:        rootLabel,
				Mountpoint:   "/",
				FSTabOptions: "defaults",
			}
			// Let the function autogenerate the name to avoid conflicts
			// with LV names from customizations.
			// Set the size to 0 and it will be adjusted by
			// EnsureDirectorySizes() and relayout().
			if _, err := payload.CreateLogicalVolume("", 0, rootfs); err != nil {
				return fmt.Errorf("error creating root logical volume: %w", err)
			}
			return nil
		case *Btrfs:
			rootName, err := genUniqueString("root", subvolNames)
			if err != nil {
				return fmt.Errorf("error creating root subvolume: %w", err)
			}
			rootsubvol := BtrfsSubvolume{
				Name:       rootName,
				Mountpoint: "/",
			}
			payload.Subvolumes = append(payload.Subvolumes, rootsubvol)
			return nil
		}
	}

	// We're going to create a root partition, so we have to ensure the default type is set.
	if defaultFsType == FS_NONE {
		return fmt.Errorf("error creating root partition: no default filesystem type")
	}

	// add a plain root partition at the end of the partition table
	rootLabel, err := genUniqueString("root", labels)
	if err != nil {
		return fmt.Errorf("error creating root partition: %w", err)
	}

	partType, err := getPartitionTypeIDfor(pt.Type, "root", architecture)
	if err != nil {
		return fmt.Errorf("error creating root partition: %w", err)
	}
	rootpart := Partition{
		Type: partType,
		Size: 0, // Set the size to 0 and it will be adjusted by EnsureDirectorySizes() and relayout()
		Payload: &Filesystem{
			Type:         defaultFsType.String(),
			Label:        rootLabel,
			Mountpoint:   "/",
			FSTabOptions: "defaults",
		},
	}
	pt.Partitions = append(pt.Partitions, rootpart)
	return nil
}

// addBootPartition creates a boot partition. The function will append the boot
// partition to the end of the existing partition table therefore it is best to
// call this function early to put boot near the front (as is conventional).
func addBootPartition(pt *PartitionTable, bootFsType FSType) error {
	if bootFsType == FS_NONE {
		return fmt.Errorf("error creating boot partition: no filesystem type")
	}

	// collect all labels to avoid conflicts
	labels := make(map[string]bool)
	_ = pt.ForEachMountable(func(mnt Mountable, path []Entity) error {
		labels[mnt.GetFSSpec().Label] = true
		return nil
	})

	bootLabel, err := genUniqueString("boot", labels)
	if err != nil {
		return fmt.Errorf("error creating boot partition: %w", err)
	}

	partType, err := getPartitionTypeIDfor(pt.Type, "boot", arch.ARCH_UNSET)
	if err != nil {
		return fmt.Errorf("error creating boot partition: %w", err)
	}
	bootPart := Partition{
		Type: partType,
		Size: 512 * datasizes.MiB,
		Payload: &Filesystem{
			Type:         bootFsType.String(),
			Label:        bootLabel,
			Mountpoint:   "/boot",
			FSTabOptions: "defaults",
		},
	}
	pt.Partitions = append(pt.Partitions, bootPart)
	return nil
}

// addPartitionsForBootMode creates partitions to satisfy the boot mode requirements:
//   - BIOS/legacy: adds a 1 MiB BIOS boot partition.
//   - UEFI: adds a 200 MiB EFI system partition.
//   - Hybrid: adds both.
//
// The function will append the new partitions to the end of the existing
// partition table therefore it is best to call this function early to put them
// near the front (as is conventional).
func addPartitionsForBootMode(pt *PartitionTable, bootMode platform.BootMode) error {
	switch bootMode {
	case platform.BOOT_LEGACY:
		// add BIOS boot partition
		part, err := mkBIOSBoot(pt.Type)
		if err != nil {
			return err
		}
		pt.Partitions = append(pt.Partitions, part)
		return nil
	case platform.BOOT_UEFI:
		// add ESP
		part, err := mkESP(200*datasizes.MiB, pt.Type)
		if err != nil {
			return err
		}
		pt.Partitions = append(pt.Partitions, part)
		return nil
	case platform.BOOT_HYBRID:
		// add both
		bios, err := mkBIOSBoot(pt.Type)
		if err != nil {
			return err
		}
		esp, err := mkESP(200*datasizes.MiB, pt.Type)
		if err != nil {
			return err
		}
		pt.Partitions = append(pt.Partitions, bios, esp)
		return nil
	case platform.BOOT_NONE:
		return nil
	default:
		return fmt.Errorf("unknown or unsupported boot mode type with enum value %d", bootMode)
	}
}

func mkBIOSBoot(ptType PartitionTableType) (Partition, error) {
	partType, err := getPartitionTypeIDfor(ptType, "bios", arch.ARCH_UNSET)
	if err != nil {
		return Partition{}, fmt.Errorf("error creating BIOS boot partition: %w", err)
	}
	return Partition{
		Size:     1 * datasizes.MiB,
		Bootable: true,
		Type:     partType,
		UUID:     BIOSBootPartitionUUID,
	}, nil
}

func mkESP(size uint64, ptType PartitionTableType) (Partition, error) {
	partType, err := getPartitionTypeIDfor(ptType, "esp", arch.ARCH_UNSET)
	if err != nil {
		return Partition{}, fmt.Errorf("error creating EFI system partition: %w", err)
	}
	return Partition{
		Size: size,
		Type: partType,
		UUID: EFISystemPartitionUUID,
		Payload: &Filesystem{
			Type:         "vfat",
			UUID:         EFIFilesystemUUID,
			Mountpoint:   "/boot/efi",
			Label:        "EFI-SYSTEM",
			FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
			FSTabFreq:    0,
			FSTabPassNo:  2,
		},
	}, nil
}

type CustomPartitionTableOptions struct {
	// PartitionTableType must be either PT_DOS or PT_GPT. Defaults to PT_GPT.
	PartitionTableType PartitionTableType

	// BootMode determines the types of boot-related partitions that are
	// automatically added, BIOS boot (legacy), ESP (UEFI), or both (hybrid).
	// If none, no boot-related partitions are created.
	BootMode platform.BootMode

	// DefaultFSType determines the filesystem type for automatically created
	// filesystems and custom mountpoints that don't specify a type.
	// None is only valid if no partitions are created and all mountpoints
	// partitions specify a type.
	// The default type is also used for the automatically created /boot
	// filesystem if it is a supported type for that fileystem. If it is not,
	// xfs is used as a fallback.
	DefaultFSType FSType

	// RequiredMinSizes defines a map of minimum sizes for specific
	// directories. These indirectly control the minimum sizes of partitions. A
	// directory with a required size will set the minimum size of the
	// partition with the mountpoint that contains the directory. Additional
	// directory requirements are additive, meaning the minimum size for a
	// mountpoint's partition is the sum of all the required directory sizes it
	// will contain.
	RequiredMinSizes map[string]uint64

	// Architecture of the hardware that will use the partition table. This is
	// used to select appropriate partition types for GPT formatted disks to
	// enable automatic discovery. It has no effect and is not required when
	// the PartitionTableType is PT_DOS.
	Architecture arch.Arch
}

// Returns the default filesystem type if the fstype is empty. If both are
// empty/none, returns an error.
func (options *CustomPartitionTableOptions) getfstype(fstype string) (string, error) {
	if fstype != "" {
		return fstype, nil
	}

	if options.DefaultFSType == FS_NONE {
		return "", fmt.Errorf("no filesystem type defined and no default set")
	}

	return options.DefaultFSType.String(), nil
}

func maybeAddBootPartition(pt *PartitionTable, disk *blueprint.DiskCustomization, defaultFSType FSType) error {
	// The boot type will be the default only if it's a supported filesystem
	// type for /boot (ext4 or xfs). Otherwise, we default to xfs.
	// FS_NONE also falls back to xfs.
	var bootFsType FSType
	switch defaultFSType {
	case FS_EXT4, FS_XFS:
		bootFsType = defaultFSType
	default:
		bootFsType = FS_XFS
	}

	if needsBoot(disk) {
		// we need a /boot partition to boot LVM or Btrfs, create boot
		// partition if it does not already exist
		if err := addBootPartition(pt, bootFsType); err != nil {
			return err
		}
	}
	return nil
}

// NewCustomPartitionTable creates a partition table based almost entirely on the disk customizations from a blueprint.
func NewCustomPartitionTable(customizations *blueprint.DiskCustomization, options *CustomPartitionTableOptions, rng *rand.Rand) (*PartitionTable, error) {
	if options == nil {
		options = &CustomPartitionTableOptions{}
	}
	if customizations == nil {
		customizations = &blueprint.DiskCustomization{}
	}

	errPrefix := "error generating partition table:"

	// validate the partitioning customizations before using them
	if err := customizations.Validate(); err != nil {
		return nil, fmt.Errorf("%s %w", errPrefix, err)
	}

	pt := &PartitionTable{}

	switch customizations.Type {
	case "dos":
		pt.Type = PT_DOS
	case "gpt":
		pt.Type = PT_GPT
	case "":
		// partition table type not specified, determine the default
		switch options.PartitionTableType {
		case PT_GPT, PT_DOS:
			pt.Type = options.PartitionTableType
		case PT_NONE:
			// default to "gpt"
			pt.Type = PT_GPT
		default:
			return nil, fmt.Errorf("%s invalid partition table type enum value: %d", errPrefix, options.PartitionTableType)
		}
	default:
		return nil, fmt.Errorf("%s invalid partition table type: %s", errPrefix, customizations.Type)
	}

	// add any partition(s) that are needed for booting (like /boot/efi)
	// if needed
	//
	// TODO: switch to ensure ESP in case customizations already include it
	if err := addPartitionsForBootMode(pt, options.BootMode); err != nil {
		return nil, fmt.Errorf("%s %w", errPrefix, err)
	}
	// add the /boot partition (if it is needed)
	if err := maybeAddBootPartition(pt, customizations, options.DefaultFSType); err != nil {
		return nil, fmt.Errorf("%s %w", errPrefix, err)
	}
	// add user customized partitions
	for _, part := range customizations.Partitions {
		if part.PartType != "" {
			// check the partition type now that we also know the partition table type
			if err := part.ValidatePartitionTypeID(pt.Type.String()); err != nil {
				return nil, fmt.Errorf("%s error validating partition type ID for %q: %w", errPrefix, part.Mountpoint, err)
			}
		}

		switch part.Type {
		case "plain", "":
			if err := addPlainPartition(pt, part, options); err != nil {
				return nil, fmt.Errorf("%s %w", errPrefix, err)
			}
		case "lvm":
			if err := addLVMPartition(pt, part, options); err != nil {
				return nil, fmt.Errorf("%s %w", errPrefix, err)
			}
		case "btrfs":
			if err := addBtrfsPartition(pt, part); err != nil {
				return nil, fmt.Errorf("%s %w", errPrefix, err)
			}
		default:
			return nil, fmt.Errorf("%s invalid partition type: %s", errPrefix, part.Type)
		}
	}

	if err := EnsureRootFilesystem(pt, options.DefaultFSType, options.Architecture); err != nil {
		return nil, fmt.Errorf("%s %w", errPrefix, err)
	}

	if len(options.RequiredMinSizes) != 0 {
		pt.EnsureDirectorySizes(options.RequiredMinSizes)
	}

	pt.relayout(customizations.MinSize)
	pt.GenerateUUIDs(rng)

	// One thing not caught by the customization validation is if a final "dos"
	// partition table has more than 4 partitions. This is not possible to
	// predict with customizations alone because it depends on the boot type
	// (which comes from the image type) which controls automatic partition
	// creation. We should therefore always check the final partition table for
	// this rule.
	if pt.Type == PT_DOS && len(pt.Partitions) > 4 {
		return nil, fmt.Errorf("%s invalid partition table: \"dos\" partition table type only supports up to 4 partitions: got %d after creating the partition table with all necessary partitions", errPrefix, len(pt.Partitions))
	}

	return pt, nil
}

func addPlainPartition(pt *PartitionTable, partition blueprint.PartitionCustomization, options *CustomPartitionTableOptions) error {
	fstype, err := options.getfstype(partition.FSType)
	if err != nil {
		return fmt.Errorf("error creating partition with mountpoint %q: %w", partition.Mountpoint, err)
	}

	partType := partition.PartType

	if partType == "" {
		// if the partition type is not specified, determine it based on the
		// mountpoint and the partition type

		var typeName string
		switch {
		case partition.Mountpoint == "/":
			typeName = "root"
		case partition.Mountpoint == "/usr":
			typeName = "usr"
		case partition.Mountpoint == "/boot":
			typeName = "boot"
		case fstype == "swap":
			typeName = "swap"
		default:
			typeName = "data"
		}

		partType, err = getPartitionTypeIDfor(pt.Type, typeName, options.Architecture)
		if err != nil {
			return fmt.Errorf("error getting partition type ID for %q: %w", partition.Mountpoint, err)
		}
	}

	var payload PayloadEntity
	switch fstype {
	case "swap":
		payload = &Swap{
			Label:        partition.Label,
			FSTabOptions: "defaults", // TODO: add customization
		}
	default:
		payload = &Filesystem{
			Type:         fstype,
			Label:        partition.Label,
			Mountpoint:   partition.Mountpoint,
			FSTabOptions: "defaults", // TODO: add customization
		}
	}

	newpart := Partition{
		Type:    partType,
		Size:    partition.MinSize,
		Payload: payload,
	}
	pt.Partitions = append(pt.Partitions, newpart)
	return nil
}

func addLVMPartition(pt *PartitionTable, partition blueprint.PartitionCustomization, options *CustomPartitionTableOptions) error {
	vgname := partition.Name
	if vgname == "" {
		// get existing volume groups and generate unique name
		existing := make(map[string]bool)
		for _, part := range pt.Partitions {
			vg, ok := part.Payload.(*LVMVolumeGroup)
			if !ok {
				continue
			}
			existing[vg.Name] = true
		}
		// unlike other unique name generation cases, here we want the first
		// name to have the 00 suffix, so we add the base to the existing set
		base := "vg"
		existing[base] = true
		uniqueName, err := genUniqueString(base, existing)
		if err != nil {
			return fmt.Errorf("error creating volume group: %w", err)
		}
		vgname = uniqueName
	}

	newvg := &LVMVolumeGroup{
		Name:        vgname,
		Description: "created via lvm2 and osbuild",
	}
	for _, lv := range partition.LogicalVolumes {
		fstype, err := options.getfstype(lv.FSType)
		if err != nil {
			return fmt.Errorf("error creating logical volume %q (%s): %w", lv.Name, lv.Mountpoint, err)
		}

		var newfs PayloadEntity
		switch fstype {
		case "swap":
			newfs = &Swap{
				Label:        lv.Label,
				FSTabOptions: "defaults", // TODO: add customization
			}
		default:
			newfs = &Filesystem{
				Type:         fstype,
				Label:        lv.Label,
				Mountpoint:   lv.Mountpoint,
				FSTabOptions: "defaults", // TODO: add customization
			}
		}
		if _, err := newvg.CreateLogicalVolume(lv.Name, lv.MinSize, newfs); err != nil {
			return fmt.Errorf("error creating logical volume %q (%s): %w", lv.Name, lv.Mountpoint, err)
		}
	}

	// create partition for volume group
	partType := partition.PartType
	if partType == "" {
		var err error
		partType, err = getPartitionTypeIDfor(pt.Type, "lvm", options.Architecture)
		if err != nil {
			return fmt.Errorf("error creating lvm partition %q: %w", vgname, err)
		}
	}

	newpart := Partition{
		Type:     partType,
		Size:     partition.MinSize,
		Bootable: false,
		Payload:  newvg,
	}
	pt.Partitions = append(pt.Partitions, newpart)
	return nil
}

func addBtrfsPartition(pt *PartitionTable, partition blueprint.PartitionCustomization) error {
	subvols := make([]BtrfsSubvolume, len(partition.Subvolumes))
	for idx, subvol := range partition.Subvolumes {
		newsubvol := BtrfsSubvolume{
			Name:       subvol.Name,
			Mountpoint: subvol.Mountpoint,
		}
		subvols[idx] = newsubvol
	}

	newvol := &Btrfs{
		Subvolumes: subvols,
	}

	// create partition for btrfs volume
	partType := partition.PartType
	if partType == "" {
		var err error
		partType, err = getPartitionTypeIDfor(pt.Type, "data", arch.ARCH_UNSET)
		if err != nil {
			return fmt.Errorf("error creating btrfs partition: %w", err)
		}
	}
	newpart := Partition{
		Type:     partType,
		Bootable: false,
		Payload:  newvol,
		Size:     partition.MinSize,
	}

	pt.Partitions = append(pt.Partitions, newpart)
	return nil
}

// Determine if a boot partition is needed based on the customizations. A boot
// partition is needed if any of the following conditions apply:
//   - / is on LVM or btrfs and /boot is not defined.
//   - / is not defined and btrfs or lvm volumes are defined.
//
// In the second case, a root partition will be created automatically on either
// btrfs or lvm.
func needsBoot(disk *blueprint.DiskCustomization) bool {
	if disk == nil {
		return false
	}

	var foundBtrfsOrLVM bool
	for _, part := range disk.Partitions {
		switch part.Type {
		case "plain", "":
			if part.Mountpoint == "/" {
				return false
			}
			if part.Mountpoint == "/boot" {
				return false
			}
		case "lvm":
			foundBtrfsOrLVM = true
			// check if any of the LVs is root
			for _, lv := range part.LogicalVolumes {
				if lv.Mountpoint == "/" {
					return true
				}
			}
		case "btrfs":
			foundBtrfsOrLVM = true
			// check if any of the subvols is root
			for _, subvol := range part.Subvolumes {
				if subvol.Mountpoint == "/" {
					return true
				}
			}
		default:
			// NOTE: invalid types should be validated elsewhere
		}
	}
	return foundBtrfsOrLVM
}
