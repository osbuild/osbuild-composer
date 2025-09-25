// Package disk contains data types and functions to define and modify
// disk-related and partition-table-related entities.
//
// The disk package is a collection of interfaces and structs that can be used
// to represent a disk image with its layout. Various concrete types, such as
// PartitionTable, Partition and Filesystem types are defined to model a given
// disk layout. These implement a collection of interfaces that can be used to
// navigate and operate on the various possible combinations of entities in a
// generic way. The entity data model is very generic so that it can represent
// all possible layouts, which can be arbitrarily complex, since technologies
// like logical volume management, LUKS2 containers and file systems, that can
// have sub-volumes, allow for complex and nested layouts.
//
// Entity and Container are the two main interfaces that are used to model the
// tree structure of a disk image layout. The other entity interfaces, such as
// Sizeable and Mountable, then describe various properties and capabilities
// of a given entity.
package disk

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
)

const (
	// Default sector size in bytes
	DefaultSectorSize = 512

	// Default grain size in bytes. The grain controls how sizes of certain
	// entities are rounded. For example, by default, partition sizes are
	// rounded to the next MiB.
	DefaultGrainBytes = uint64(1048576) // 1 MiB

	// GUIDs (partition types) for partitions on GPT disks
	// The SD_GPT name next to each constant is the partition type shown in
	// systemd-gpt-auto-generator(8) (if present)
	// See also
	// - https://www.freedesktop.org/wiki/Specifications/DiscoverablePartitionsSpec/
	// - https://uapi-group.org/specifications/specs/discoverable_partitions_specification/
	BIOSBootPartitionGUID  = "21686148-6449-6E6F-744E-656564454649"
	FilesystemDataGUID     = "0FC63DAF-8483-4772-8E79-3D69D8477DE4" // SD_GPT_LINUX_GENERIC
	EFISystemPartitionGUID = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B" // SD_GPT_ESP
	LVMPartitionGUID       = "E6D6D379-F507-44C2-A23C-238F2A3DF928"
	PRePartitionGUID       = "9E1A2D38-C612-4316-AA26-8B49521E5A8B"
	SwapPartitionGUID      = "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F" // SD_GPT_SWAP
	XBootLDRPartitionGUID  = "BC13C2FF-59E6-4262-A352-B275FD6F7172" // SD_GPT_XBOOTLDR

	RootPartitionX86_64GUID  = "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709" // SD_GPT_ROOT_X86_64
	RootPartitionAarch64GUID = "B921B045-1DF0-41C3-AF44-4C6F280D3FAE" // SD_GPT_ROOT_ARM64
	RootPartitionPpc64leGUID = "C31C45E6-3F39-412E-80FB-4809C4980599" // SD_GPT_ROOT_PPC64_LE
	RootPartitionS390xGUID   = "5EEAD9A9-FE09-4A1E-A1D7-520D00531306" // SD_GPT_ROOT_S390X

	UsrPartitionX86_64GUID  = "8484680C-9521-48C6-9C11-B0720656F69E" // SD_GPT_USR_X86_64
	UsrPartitionAarch64GUID = "B0E01050-EE5F-4390-949A-9101B17104E9" // SD_GPT_USR_ARM64
	UsrPartitionPpc64leGUID = "15BB03AF-77E7-4D4A-B12B-C0D084F7491C" // SD_GPT_USR_PPC64_LE
	UsrPartitionS390xGUID   = "8A4F5770-50AA-4ED3-874A-99B710DB6FEA" // SD_GPT_USR_S390X

	// Partition type IDs for DOS disks

	// Partition type ID for BIOS boot partition on dos.
	// Type ID is for 'empty'.
	// TODO: drop this completely when we convert the bios BOOT space to a
	// partitionless gap/offset.
	BIOSBootPartitionDOSID = "00"

	// Partition type ID for any native Linux filesystem on dos
	FilesystemLinuxDOSID = "83"

	// FAT16BDOSID used for the ESP-System partition
	FAT16BDOSID = "06"

	// Partition type ID for LVM on dos
	LVMPartitionDOSID = "8e"

	// Partition type ID for ESP on dos
	EFISystemPartitionDOSID = "ef"

	// Partition type ID for swap
	SwapPartitionDOSID = "82"

	// Partition type ID for PRep on dos
	PRepPartitionDOSID = "41"

	// static UUIDs for partitions and filesystems
	// NOTE(akoutsou): These are unnecessary and have stuck around since the
	// beginning where (I believe) the goal was to have predictable,
	// reproducible partition tables. They might be removed soon in favour of
	// proper, random UUIDs, with reproducibility being controlled by fixing
	// rng seeds.
	BIOSBootPartitionUUID  = "FAC7F1FB-3E8D-4137-A512-961DE09A5549"
	RootPartitionUUID      = "6264D520-3FB9-423F-8AB8-7A0A8E3D3562"
	DataPartitionUUID      = "CB07C243-BC44-4717-853E-28852021225B"
	EFISystemPartitionUUID = "68B2905B-DF3E-4FB3-80FA-49D1E773AA33"

	EFIFilesystemUUID = "7B77-95E7"

	ESPFstabOptions = "defaults,uid=0,gid=0,umask=077,shortname=winnt"
)

func getPartitionTypeIDfor(ptType PartitionTableType, partTypeName string, architecture arch.Arch) (string, error) {
	switch ptType {
	case PT_DOS:
		switch partTypeName {
		case "bios":
			return BIOSBootPartitionDOSID, nil
		case "data", "boot", "root", "usr":
			return FilesystemLinuxDOSID, nil
		case "esp":
			return EFISystemPartitionDOSID, nil
		case "lvm":
			return LVMPartitionDOSID, nil
		case "ppc_prep":
			return PRepPartitionDOSID, nil
		case "swap":
			return SwapPartitionDOSID, nil
		default:
			return "", fmt.Errorf("unknown or unsupported partition type name: %s", partTypeName)
		}
	case PT_GPT:
		switch partTypeName {
		case "bios":
			return BIOSBootPartitionGUID, nil
		case "boot":
			return XBootLDRPartitionGUID, nil
		case "data":
			return FilesystemDataGUID, nil
		case "esp":
			return EFISystemPartitionGUID, nil
		case "lvm":
			return LVMPartitionGUID, nil
		case "ppc_prep":
			return PRePartitionGUID, nil
		case "swap":
			return SwapPartitionGUID, nil
		case "root":
			switch architecture {
			case arch.ARCH_X86_64:
				return RootPartitionX86_64GUID, nil
			case arch.ARCH_AARCH64:
				return RootPartitionAarch64GUID, nil
			case arch.ARCH_PPC64LE:
				return RootPartitionPpc64leGUID, nil
			case arch.ARCH_S390X:
				return RootPartitionS390xGUID, nil
			case arch.ARCH_UNSET:
				return "", fmt.Errorf("architecture must be specified for selecting GUID for %q partition", partTypeName)
			default:
				return "", fmt.Errorf("unknown or unsupported architecture enum value: %d", architecture)
			}
		case "usr":
			switch architecture {
			case arch.ARCH_X86_64:
				return UsrPartitionX86_64GUID, nil
			case arch.ARCH_AARCH64:
				return UsrPartitionAarch64GUID, nil
			case arch.ARCH_PPC64LE:
				return UsrPartitionPpc64leGUID, nil
			case arch.ARCH_S390X:
				return UsrPartitionS390xGUID, nil
			case arch.ARCH_UNSET:
				return "", fmt.Errorf("architecture must be specified for selecting GUID for %q partition", partTypeName)
			default:
				return "", fmt.Errorf("unknown or unsupported architecture enum value: %d", architecture)
			}
		default:
			return "", fmt.Errorf("unknown or unsupported partition type name: %s", partTypeName)
		}
	default:
		return "", fmt.Errorf("unknown or unsupported partition table enum: %d", ptType)
	}
}

// FSType is the filesystem type enum.
//
// There should always be one value for each filesystem type supported by
// osbuild stages (stages/org.osbuild.mkfs.*) and the unset/none value.
type FSType uint64

const (
	FS_NONE FSType = iota
	FS_VFAT
	FS_EXT4
	FS_XFS
	FS_BTRFS
)

func (f FSType) String() string {
	switch f {
	case FS_NONE:
		return ""
	case FS_VFAT:
		return "vfat"
	case FS_EXT4:
		return "ext4"
	case FS_XFS:
		return "xfs"
	case FS_BTRFS:
		return "btrfs"
	default:
		panic(fmt.Sprintf("unknown or unsupported filesystem type with enum value %d", f))
	}
}

func (f *FSType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	new, err := NewFSType(s)
	if err != nil {
		return err
	}
	*f = new
	return nil
}

func (f *FSType) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(f, unmarshal)
}

func NewFSType(s string) (FSType, error) {
	switch s {
	case "":
		return FS_NONE, nil
	case "vfat":
		return FS_VFAT, nil
	case "ext4":
		return FS_EXT4, nil
	case "xfs":
		return FS_XFS, nil
	case "btrfs":
		return FS_BTRFS, nil
	default:
		return FS_NONE, fmt.Errorf("unknown or unsupported filesystem type name: %s", s)
	}
}

// PartitionTableType is the partition table type enum.
type PartitionTableType uint64

const (
	PT_NONE PartitionTableType = iota
	PT_DOS
	PT_GPT
)

func (t PartitionTableType) String() string {
	switch t {
	case PT_NONE:
		return ""
	case PT_DOS:
		return "dos"
	case PT_GPT:
		return "gpt"
	default:
		panic(fmt.Sprintf("unknown or unsupported partition table type with enum value %d", t))
	}
}

func (t PartitionTableType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t PartitionTableType) MarshalYAML() (interface{}, error) {
	return t.String(), nil
}

func (t *PartitionTableType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	new, err := NewPartitionTableType(s)
	if err != nil {
		return err
	}
	*t = new
	return nil
}

func (t *PartitionTableType) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(t, unmarshal)
}

func NewPartitionTableType(s string) (PartitionTableType, error) {
	switch s {
	case "":
		return PT_NONE, nil
	case "dos":
		return PT_DOS, nil
	case "gpt":
		return PT_GPT, nil
	default:
		return PT_NONE, fmt.Errorf("unknown or unsupported partition table type name: %s", s)
	}
}

// Entity is the base interface for all disk-related entities.
type Entity interface {
	// Clone returns a deep copy of the entity.
	Clone() Entity
}

// PayloadEntity is an entity that can be used as a Payload for a Container.
type PayloadEntity interface {
	Entity

	// EntityName is the type name of the Entity, used for marshaling
	EntityName() string
}

var payloadEntityMap = map[string]reflect.Type{}

// Container is the interface for entities that can contain other entities.
// Together with the base Entity interface this allows to model a generic
// entity tree of theoretically arbitrary depth and width.
type Container interface {
	Entity

	// GetItemCount returns the number of actual child entities.
	GetItemCount() uint

	// GetChild returns the child entity at the given index.
	GetChild(n uint) Entity
}

// Sizeable is implemented by entities that carry size information.
type Sizeable interface {
	// EnsureSize will resize the entity to the given size in case
	// it is currently smaller. Returns if the size was changed.
	EnsureSize(size uint64) bool

	// GetSize returns the size of the entity in bytes.
	GetSize() uint64
}

// A Mountable entity is an entity that can be mounted.
type Mountable interface {

	// GetMountPoint returns the path of the mount point.
	GetMountpoint() string

	FSTabEntity
}

// FSTabEntity describes any entity that can appear in the fstab file.
type FSTabEntity interface {
	// FSSpec for the entity (UUID and Label); the first field of fstab(5).
	GetFSSpec() FSSpec

	// The mount point (target) for a filesystem or "none" for swap areas; the second field of fstab(5).
	GetFSFile() string

	// The type of the filesystem or swap for swap areas; the third field of fstab(5).
	GetFSType() string

	// The mount options, freq, and passno for the entity; the fourth fifth, and sixth fields of fstab(5) respectively.
	GetFSTabOptions() (FSTabOptions, error)
}

// A MountpointCreator is a container that is able to create new volumes.
//
// CreateMountpoint creates a new mountpoint with the given size and
// default filesystem and returns the entity that represents the new
// mountpoint. The defaultFs is only a hint and can be ignored by
// e.g. btrfs subvolumes.
type MountpointCreator interface {
	CreateMountpoint(mountpoint, defaultFs string, size uint64) (Entity, error)

	// AlignUp will align the given bytes according to the
	// requirements of the container type.
	AlignUp(size uint64) uint64
}

// A UniqueEntity is an entity that can be uniquely identified via a UUID.
//
// GenUUID generates a UUID for the entity if it does not yet have one.
type UniqueEntity interface {
	Entity
	GenUUID(rng *rand.Rand)
}

// VolumeContainer is a specific container that contains volume entities
type VolumeContainer interface {

	// MetadataSize returns the size of the container's metadata (in
	// bytes), i.e. the storage space that needs to be reserved for
	// the container itself, in contrast to the data it contains.
	MetadataSize() uint64

	// minSize returns the size for the VolumeContainer that is either the
	// provided desired size value or the sum of all children if that is
	// larger. It will also add any space required for metadata. The returned
	// value should, at minimum, be large enough to fit all the children, their
	// metadata, and the VolumeContainer's metadata. In other words, the
	// VolumeContainer's size, or its parent size, will be able to hold the
	// VolumeContainer if it is created with the exact size returned by the
	// function.
	minSize(size uint64) uint64
}

// FSSpec for a filesystem (UUID and Label); the first field of fstab(5)
type FSSpec struct {
	UUID  string
	Label string
}

type FSTabOptions struct {
	// The fourth field of fstab(5); fs_mntops
	MntOps string
	// The fifth field of fstab(5); fs_freq
	Freq uint64
	// The sixth field of fstab(5); fs_passno
	PassNo uint64
}

// ReadOnly returns true if the filesystem is mounted read-only.
func (o FSTabOptions) ReadOnly() bool {
	opts := strings.Split(o.MntOps, ",")

	// filesystem is mounted read-only if:
	// - there's ro (because rw is the default)
	// - AND there's no rw (because rw overrides ro)
	return slices.Contains(opts, "ro") && !slices.Contains(opts, "rw")
}

// uuid generator helpers

// GeneratesnewRandomUUIDFromReader generates a new random UUID (version
// 4 using) via the given random number generator.
func newRandomUUIDFromReader(r io.Reader) (uuid.UUID, error) {
	var id uuid.UUID
	_, err := io.ReadFull(r, id[:])
	if err != nil {
		return uuid.Nil, err
	}
	id[6] = (id[6] & 0x0f) | 0x40 // Version 4
	id[8] = (id[8] & 0x3f) | 0x80 // Variant is 10
	return id, nil
}

// NewVolIDFromRand creates a random 32 bit hex string to use as a volume ID
// for FAT filesystems. It ensures there's a dash in the middle and the string
// is uppercased.
func NewVolIDFromRand(r *rand.Rand) string {
	volid := make([]byte, 4)
	len, _ := r.Read(volid)
	if len != 4 {
		panic("expected four random bytes")
	}
	asHex := strings.ToUpper(hex.EncodeToString(volid))
	return fmt.Sprintf("%s-%s", asHex[:4], asHex[4:])
}

// genUniqueString returns a string based on base that does does not exist in
// the existing set. If the base itself does not exist, it is returned as is,
// otherwise a two digit number is added and incremented until a unique string
// is found.
// This function is mimicking what blivet does for avoiding name collisions.
// See blivet/blivet.py#L1060 commit 2eb4bd4
func genUniqueString(base string, existing map[string]bool) (string, error) {
	if !existing[base] {
		return base, nil
	}

	for i := 0; i < 100; i++ {
		uniq := fmt.Sprintf("%s%02d", base, i)
		if !existing[uniq] {
			return uniq, nil
		}
	}

	return "", fmt.Errorf("name collision: could not generate unique version of %q", base)
}
