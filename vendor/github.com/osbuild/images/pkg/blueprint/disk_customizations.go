package blueprint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/pathpolicy"
)

type DiskCustomization struct {
	// Type of the partition table: gpt or dos.
	// Optional, the default depends on the distro and image type.
	Type       string
	MinSize    uint64
	Partitions []PartitionCustomization
}

type diskCustomizationMarshaler struct {
	Type       string                   `json:"type,omitempty" toml:"type,omitempty"`
	MinSize    datasizes.Size           `json:"minsize,omitempty" toml:"minsize,omitempty"`
	Partitions []PartitionCustomization `json:"partitions,omitempty" toml:"partitions,omitempty"`
}

func (dc *DiskCustomization) UnmarshalJSON(data []byte) error {
	var dcm diskCustomizationMarshaler
	if err := json.Unmarshal(data, &dcm); err != nil {
		return err
	}
	dc.Type = dcm.Type
	dc.MinSize = dcm.MinSize.Uint64()
	dc.Partitions = dcm.Partitions

	return nil
}

func (dc *DiskCustomization) UnmarshalTOML(data any) error {
	return unmarshalTOMLviaJSON(dc, data)
}

// PartitionCustomization defines a single partition on a disk. The Type
// defines the kind of "payload" for the partition: plain, lvm, or btrfs.
//   - plain: the payload will be a filesystem on a partition (e.g. xfs, ext4).
//     See [FilesystemTypedCustomization] for extra fields.
//   - lvm: the payload will be an LVM volume group. See [VGCustomization] for
//     extra fields
//   - btrfs: the payload will be a btrfs volume. See
//     [BtrfsVolumeCustomization] for extra fields.
type PartitionCustomization struct {
	// The type of payload for the partition (optional, defaults to "plain").
	Type string `json:"type" toml:"type"`

	// Minimum size of the partition that contains the filesystem (for "plain"
	// filesystem), volume group ("lvm"), or btrfs volume ("btrfs"). The final
	// size of the partition will be larger than the minsize if the sum of the
	// contained volumes (logical volumes or subvolumes) is larger. In
	// addition, certain mountpoints have required minimum sizes. See
	// https://osbuild.org/docs/user-guide/partitioning for more details.
	// (optional, defaults depend on payload and mountpoints).
	MinSize uint64 `json:"minsize" toml:"minsize"`

	// The partition type GUID for GPT partitions. For DOS partitions, this
	// field can be used to set the (2 hex digit) partition type.
	// If not set, the type will be automatically set based on the mountpoint
	// or the payload type.
	PartType string `json:"part_type,omitempty" toml:"part_type,omitempty"`

	// The partition label for GPT partitions, not supported for dos partitions.
	// Note: This is not the same as the label, which can be set in "Label"
	PartLabel string `json:"part_label,omitempty" toml:"part_label,omitempty"`

	// The partition GUID for GPT partitions, not supported for dos partitions.
	// Note: This is the unique uuid, not the type guid, that is PartType
	PartUUID string `json:"part_uuid,omitempty" toml:"part_uuid,omitempty"`

	BtrfsVolumeCustomization

	VGCustomization

	FilesystemTypedCustomization
}

// A filesystem on a plain partition or LVM logical volume.
// Note the differences from [FilesystemCustomization]:
//   - Adds a label.
//   - Adds a filesystem type (fs_type).
//   - Does not define a size. The size is defined by its container: a
//     partition ([PartitionCustomization]) or LVM logical volume
//     ([LVCustomization]).
//
// Setting the FSType to "swap" creates a swap area (and the Mountpoint must be
// empty).
type FilesystemTypedCustomization struct {
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
	Label      string `json:"label,omitempty" toml:"label,omitempty"`
	FSType     string `json:"fs_type,omitempty" toml:"fs_type,omitempty"`
}

// An LVM volume group with one or more logical volumes.
type VGCustomization struct {
	// Volume group name (optional, default will be automatically generated).
	Name           string            `json:"name" toml:"name"`
	LogicalVolumes []LVCustomization `json:"logical_volumes,omitempty" toml:"logical_volumes,omitempty"`
}

type LVCustomization struct {
	// Logical volume name
	Name string `json:"name,omitempty" toml:"name,omitempty"`

	// Minimum size of the logical volume
	MinSize uint64 `json:"minsize,omitempty" toml:"minsize,omitempty"`

	FilesystemTypedCustomization
}

// Custom JSON unmarshaller for LVCustomization for handling the conversion of
// data sizes (minsize) expressed as strings to uint64.
func (lv *LVCustomization) UnmarshalJSON(data []byte) error {
	var lvAnySize struct {
		Name    string `json:"name,omitempty" toml:"name,omitempty"`
		MinSize any    `json:"minsize,omitempty" toml:"minsize,omitempty"`
		FilesystemTypedCustomization
	}
	if err := json.Unmarshal(data, &lvAnySize); err != nil {
		return err
	}

	lv.Name = lvAnySize.Name
	lv.FilesystemTypedCustomization = lvAnySize.FilesystemTypedCustomization

	if lvAnySize.MinSize == nil {
		return fmt.Errorf("minsize is required")
	}
	size, err := decodeSize(lvAnySize.MinSize)
	if err != nil {
		return err
	}
	lv.MinSize = size

	return nil
}

// A btrfs volume consisting of one or more subvolumes.
type BtrfsVolumeCustomization struct {
	Subvolumes []BtrfsSubvolumeCustomization
}

type BtrfsSubvolumeCustomization struct {
	// The name of the subvolume, which defines the location (path) on the
	// root volume (required).
	// See https://btrfs.readthedocs.io/en/latest/Subvolumes.html
	Name string `json:"name" toml:"name"`

	// Mountpoint for the subvolume.
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
}

// Custom JSON unmarshaller that first reads the value of the "type" field and
// then deserialises the whole object into a struct that only contains the
// fields valid for that partition type. This ensures that no fields are set
// for the substructure of a different type than the one defined in the "type"
// fields.
func (v *PartitionCustomization) UnmarshalJSON(data []byte) error {
	errPrefix := "JSON unmarshal:"
	var typeSniffer struct {
		Type      string `json:"type"`
		MinSize   any    `json:"minsize"`
		PartType  string `json:"part_type"`
		PartLabel string `json:"part_label"`
		PartUUID  string `json:"part_uuid"`
	}
	if err := json.Unmarshal(data, &typeSniffer); err != nil {
		return fmt.Errorf("%s %w", errPrefix, err)
	}

	partType := "plain"
	if typeSniffer.Type != "" {
		partType = typeSniffer.Type
	}

	switch partType {
	case "plain":
		if err := decodePlain(v, data); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	case "btrfs":
		if err := decodeBtrfs(v, data); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	case "lvm":
		if err := decodeLVM(v, data); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	default:
		return fmt.Errorf("%s unknown partition type: %s", errPrefix, partType)
	}

	v.Type = partType
	v.PartType = typeSniffer.PartType
	v.PartLabel = typeSniffer.PartLabel
	v.PartUUID = typeSniffer.PartUUID

	if typeSniffer.MinSize == nil {
		return fmt.Errorf("minsize is required")
	}

	minsize, err := decodeSize(typeSniffer.MinSize)
	if err != nil {
		return fmt.Errorf("%s error decoding minsize for partition: %w", errPrefix, err)
	}
	v.MinSize = minsize

	return nil
}

// decodePlain decodes the data into a struct that only embeds the
// FilesystemCustomization with DisallowUnknownFields. This ensures that when
// the type is "plain", none of the fields for btrfs or lvm are used.
func decodePlain(v *PartitionCustomization, data []byte) error {
	var plain struct {
		// Type, minsize, and part_* are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type      string `json:"type"`
		MinSize   any    `json:"minsize"`
		PartType  string `json:"part_type"`
		PartLabel string `json:"part_label"`
		PartUUID  string `json:"part_uuid"`
		FilesystemTypedCustomization
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&plain)
	if err != nil {
		return fmt.Errorf("error decoding partition with type \"plain\": %w", err)
	}

	v.FilesystemTypedCustomization = plain.FilesystemTypedCustomization
	return nil
}

// decodeBtrfs decodes the data into a struct that only embeds the
// BtrfsVolumeCustomization with DisallowUnknownFields. This ensures that when
// the type is btrfs, none of the fields for plain or lvm are used.
func decodeBtrfs(v *PartitionCustomization, data []byte) error {
	var btrfs struct {
		// Type, minsize, and part_* are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type      string `json:"type"`
		MinSize   any    `json:"minsize"`
		PartType  string `json:"part_type"`
		PartLabel string `json:"part_label"`
		PartUUID  string `json:"part_uuid"`
		BtrfsVolumeCustomization
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&btrfs)
	if err != nil {
		return fmt.Errorf("error decoding partition with type \"btrfs\": %w", err)
	}

	v.BtrfsVolumeCustomization = btrfs.BtrfsVolumeCustomization
	return nil
}

// decodeLVM decodes the data into a struct that only embeds the
// VGCustomization with DisallowUnknownFields. This ensures that when the type
// is lvm, none of the fields for plain or btrfs are used.
func decodeLVM(v *PartitionCustomization, data []byte) error {
	var vg struct {
		// Type, minsize, and part_* are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type      string `json:"type"`
		MinSize   any    `json:"minsize"`
		PartType  string `json:"part_type"`
		PartLabel string `json:"part_label"`
		PartUUID  string `json:"part_uuid"`
		VGCustomization
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&vg); err != nil {
		return fmt.Errorf("error decoding partition with type \"lvm\": %w", err)
	}

	v.VGCustomization = vg.VGCustomization
	return nil
}

// Custom TOML unmarshaller that first reads the value of the "type" field and
// then deserialises the whole object into a struct that only contains the
// fields valid for that partition type. This ensures that no fields are set
// for the substructure of a different type than the one defined in the "type"
// fields.
func (v *PartitionCustomization) UnmarshalTOML(data any) error {
	errPrefix := "TOML unmarshal:"
	d, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("%s customizations.partition is not an object", errPrefix)
	}

	partType := "plain"
	if typeField, ok := d["type"]; ok {
		typeStr, ok := typeField.(string)
		if !ok {
			return fmt.Errorf("%s type must be a string, got \"%v\" of type %T", errPrefix, typeField, typeField)
		}
		partType = typeStr
	}

	// serialise the data to JSON and reuse the subobject decoders
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%s error while decoding partition customization: %w", errPrefix, err)
	}
	switch partType {
	case "plain":
		if err := decodePlain(v, dataJSON); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	case "btrfs":
		if err := decodeBtrfs(v, dataJSON); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	case "lvm":
		if err := decodeLVM(v, dataJSON); err != nil {
			return fmt.Errorf("%s %w", errPrefix, err)
		}
	default:
		return fmt.Errorf("%s unknown partition type: %s", errPrefix, partType)
	}

	v.Type = partType

	minsizeField, ok := d["minsize"]
	if !ok {
		return fmt.Errorf("minsize is required")
	}
	minsize, err := decodeSize(minsizeField)
	if err != nil {
		return fmt.Errorf("%s error decoding minsize for partition: %w", errPrefix, err)
	}
	v.MinSize = minsize

	return nil
}

// Validate checks for customization combinations that are generally not
// supported or can create conflicts, regardless of specific distro or image
// type policies. The validator ensures all of the following properties:
//   - All mountpoints are valid
//   - All mountpoints are unique
//   - All LVM volume group names are unique
//   - All LVM logical volume names are unique within a given volume group
//   - All btrfs subvolume names are unique within a given btrfs volume
//   - All btrfs subvolume names are valid and non-empty
//   - All filesystems are valid for their mountpoints (e.g. xfs or ext4 for /boot)
//   - No LVM logical volume has an invalid mountpoint (/boot or /boot/efi)
//   - Plain filesystem types are valid for the partition type
//   - All non-empty properties are valid for the partition type (e.g.
//     LogicalVolumes is empty when the type is "plain" or "btrfs")
//   - Filesystems with FSType set to "none" or "swap" do not specify a mountpoint.
//
// Note that in *addition* consumers should also call
// ValidateLayoutConstraints() to validate that the policy for disk
// customizations is met.
func (p *DiskCustomization) Validate() error {
	if p == nil {
		return nil
	}

	switch p.Type {
	case "gpt", "":
	case "dos":
		// dos/mbr only supports 4 partitions
		// Unfortunately, at this stage it's unknown whether we will need extra
		// partitions (bios boot, root, esp), so this check is just to catch
		// obvious invalid customizations early. The final partition table is
		// checked after it's created.
		if len(p.Partitions) > 4 {
			return fmt.Errorf("invalid partitioning customizations: \"dos\" partition table type only supports up to 4 partitions: got %d", len(p.Partitions))
		}
	default:
		return fmt.Errorf("unknown partition table type: %s (valid: gpt, dos)", p.Type)
	}

	mountpoints := make(map[string]bool)
	vgnames := make(map[string]bool)
	var errs []error
	for _, part := range p.Partitions {
		if err := part.ValidatePartitionTypeID(p.Type); err != nil {
			errs = append(errs, err)
		}
		if err := part.ValidatePartitionID(p.Type); err != nil {
			errs = append(errs, err)
		}
		if err := part.ValidatePartitionLabel(p.Type); err != nil {
			errs = append(errs, err)
		}
		switch part.Type {
		case "plain", "":
			errs = append(errs, part.validatePlain(mountpoints))
		case "lvm":
			errs = append(errs, part.validateLVM(mountpoints, vgnames))
		case "btrfs":
			errs = append(errs, part.validateBtrfs(mountpoints))
		default:
			errs = append(errs, fmt.Errorf("unknown partition type: %s", part.Type))
		}
	}

	// will discard all nil errors
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid partitioning customizations:\n%w", err)
	}
	return nil
}

func validateMountpoint(path string) error {
	if path == "" {
		return fmt.Errorf("mountpoint is empty")
	}

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("mountpoint %q is not an absolute path", path)
	}

	if cleanPath := filepath.Clean(path); path != cleanPath {
		return fmt.Errorf("mountpoint %q is not a canonical path (did you mean %q?)", path, cleanPath)
	}

	return nil
}

// ValidateLayoutConstraints checks that at most one LVM Volume Group or btrfs
// volume is defined. Returns an error if both LVM and btrfs are set and if
// either has more than one element.
//
// Note that this is a *policy* validation, in theory the "disk" code
// does support the constraints but we choose not to allow them for
// now.  Each consumer of "DiskCustomization" should call this
// *unless* it's very low-level and not end-user-facing.
func (p *DiskCustomization) ValidateLayoutConstraints() error {
	if p == nil {
		return nil
	}

	var btrfsVols, lvmVGs uint
	for _, part := range p.Partitions {
		switch part.Type {
		case "lvm":
			lvmVGs++
		case "btrfs":
			btrfsVols++
		}
		if lvmVGs > 0 && btrfsVols > 0 {
			return fmt.Errorf("btrfs and lvm partitioning cannot be combined")
		}
	}

	if btrfsVols > 1 {
		return fmt.Errorf("multiple btrfs volumes are not yet supported")
	}

	if lvmVGs > 1 {
		return fmt.Errorf("multiple LVM volume groups are not yet supported")
	}

	return nil
}

// Check that the fs type is valid for the mountpoint.
func validateFilesystemType(path, fstype string) error {
	badfsMsgFmt := "unsupported filesystem type for %q: %s"
	switch path {
	case "/boot":
		switch fstype {
		case "xfs", "ext4":
		default:
			return fmt.Errorf(badfsMsgFmt, path, fstype)
		}
	case "/boot/efi":
		switch fstype {
		case "vfat":
		default:
			return fmt.Errorf(badfsMsgFmt, path, fstype)
		}
	}
	return nil
}

// These mountpoints must be on a plain partition (i.e. not on LVM or btrfs).
var plainOnlyMountpoints = []string{
	"/boot",
	"/boot/efi", // not allowed by our global policies, but that might change
}

var validPlainFSTypes = []string{
	"ext4",
	"vfat",
	"xfs",
}

// exactly 2 hex digits
var validDosPartitionType = regexp.MustCompile(`^[0-9a-fA-F]{2}$`)

// ValidatePartitionTypeID returns an error if the partition type ID is not
// valid given the partition table type. If the partition table type is an
// empty string, the function returns an error only if the partition type ID is
// invalid for both gpt and dos partition tables.
func (p *PartitionCustomization) ValidatePartitionTypeID(ptType string) error {
	// Empty PartType is fine, it will be selected automatically
	if p.PartType == "" {
		return nil
	}

	_, uuidErr := uuid.Parse(p.PartType)
	validDosType := validDosPartitionType.MatchString(p.PartType)

	switch ptType {
	case "gpt":
		if uuidErr != nil {
			return fmt.Errorf("invalid partition part_type %q for partition table type %q (must be a valid UUID): %w", p.PartType, ptType, uuidErr)
		}
	case "dos":
		if !validDosType {
			return fmt.Errorf("invalid partition part_type %q for partition table type %q (must be a 2-digit hex number)", p.PartType, ptType)
		}
	case "":
		// We don't know the partition table type yet, the fallback is controlled
		// by the CustomPartitionTableOptions, so return an error if it fails both.
		if uuidErr != nil && !validDosType {
			return fmt.Errorf("invalid part_type %q: must be a valid UUID for GPT partition tables or a 2-digit hex number for DOS partition tables", p.PartType)
		}
	default:
		// ignore: handled elsewhere
	}

	return nil
}

// ValidatePartitionID returns an error if the partition ID is not
// valid given the partition table type. If the partition table type is an
// empty string, the function returns an error only if the partition type ID is
// invalid for both gpt and dos partition tables.
func (p *PartitionCustomization) ValidatePartitionID(ptType string) error {
	// Empty PartUUID is fine, it will be selected automatically if needed
	if p.PartUUID == "" {
		return nil
	}

	if ptType == "dos" {
		return fmt.Errorf("part_type is not supported for dos partition tables")
	}

	_, uuidErr := uuid.Parse(p.PartUUID)
	if uuidErr != nil {
		return fmt.Errorf("invalid partition part_uuid %q (must be a valid UUID): %w", p.PartUUID, uuidErr)
	}

	return nil
}

// ValidatePartitionID returns an error if the partition ID is not
// valid given the partition table type.
func (p *PartitionCustomization) ValidatePartitionLabel(ptType string) error {
	// Empty PartLabel is fine
	if p.PartLabel == "" {
		return nil
	}

	if ptType == "dos" {
		return fmt.Errorf("part_label is not supported for dos partition tables")
	}

	// GPT Labels are up to 36 utf-16 chars
	if len(utf16.Encode([]rune(p.PartLabel))) > 36 {
		return fmt.Errorf("part_label is not a valid GPT label, it is too long")
	}

	return nil
}

func (p *PartitionCustomization) validatePlain(mountpoints map[string]bool) error {
	if p.FSType == "none" {
		// make sure the mountpoint is empty and return
		if p.Mountpoint != "" {
			return fmt.Errorf("mountpoint for none partition must be empty (got %q)", p.Mountpoint)
		}
		return nil
	}

	if p.FSType == "swap" {
		// make sure the mountpoint is empty and return
		if p.Mountpoint != "" {
			return fmt.Errorf("mountpoint for swap partition must be empty (got %q)", p.Mountpoint)
		}
		return nil
	}

	if err := validateMountpoint(p.Mountpoint); err != nil {
		return err
	}
	if mountpoints[p.Mountpoint] {
		return fmt.Errorf("duplicate mountpoint %q in partitioning customizations", p.Mountpoint)
	}
	// TODO: allow empty fstype with default from distro
	if !slices.Contains(validPlainFSTypes, p.FSType) {
		return fmt.Errorf("unknown or invalid filesystem type (fs_type) for mountpoint %q: %s", p.Mountpoint, p.FSType)
	}
	if err := validateFilesystemType(p.Mountpoint, p.FSType); err != nil {
		return err
	}

	mountpoints[p.Mountpoint] = true
	return nil
}

func (p *PartitionCustomization) validateLVM(mountpoints, vgnames map[string]bool) error {
	if p.Name != "" && vgnames[p.Name] { // VGs with no name get autogenerated names
		return fmt.Errorf("duplicate LVM volume group name %q in partitioning customizations", p.Name)
	}

	// check for invalid property usage
	if len(p.Subvolumes) > 0 {
		return fmt.Errorf("subvolumes defined for LVM volume group (partition type \"lvm\")")
	}

	if p.Label != "" {
		return fmt.Errorf("label %q defined for LVM volume group (partition type \"lvm\")", p.Label)
	}

	vgnames[p.Name] = true
	lvnames := make(map[string]bool)
	for _, lv := range p.LogicalVolumes {
		if lv.Name != "" && lvnames[lv.Name] { // LVs with no name get autogenerated names
			return fmt.Errorf("duplicate LVM logical volume name %q in volume group %q in partitioning customizations", lv.Name, p.Name)
		}
		lvnames[lv.Name] = true

		if lv.FSType == "swap" {
			// make sure the mountpoint is empty and return
			if lv.Mountpoint != "" {
				return fmt.Errorf("mountpoint for swap logical volume with name %q in volume group %q must be empty", lv.Name, p.Name)
			}
			return nil
		}
		if err := validateMountpoint(lv.Mountpoint); err != nil {
			return fmt.Errorf("invalid logical volume customization: %w", err)
		}
		if mountpoints[lv.Mountpoint] {
			return fmt.Errorf("duplicate mountpoint %q in partitioning customizations", lv.Mountpoint)
		}
		mountpoints[lv.Mountpoint] = true

		if slices.Contains(plainOnlyMountpoints, lv.Mountpoint) {
			return fmt.Errorf("invalid mountpoint %q for logical volume", lv.Mountpoint)
		}

		// TODO: allow empty fstype with default from distro
		if !slices.Contains(validPlainFSTypes, lv.FSType) {
			return fmt.Errorf("unknown or invalid filesystem type (fs_type) for logical volume with mountpoint %q: %s", lv.Mountpoint, lv.FSType)
		}
	}
	return nil
}

func (p *PartitionCustomization) validateBtrfs(mountpoints map[string]bool) error {
	if p.Mountpoint != "" {
		return fmt.Errorf(`"mountpoint" is not supported for btrfs volumes (only subvolumes can have mountpoints)`)
	}

	if len(p.Subvolumes) == 0 {
		return fmt.Errorf("btrfs volume requires subvolumes")
	}

	if len(p.LogicalVolumes) > 0 {
		return fmt.Errorf("LVM logical volumes defined for btrfs volume (partition type \"btrfs\")")
	}

	subvolnames := make(map[string]bool)
	for _, subvol := range p.Subvolumes {
		if subvol.Name == "" {
			return fmt.Errorf("btrfs subvolume with empty name in partitioning customizations")
		}
		if subvolnames[subvol.Name] {
			return fmt.Errorf("duplicate btrfs subvolume name %q in partitioning customizations", subvol.Name)
		}
		subvolnames[subvol.Name] = true

		if err := validateMountpoint(subvol.Mountpoint); err != nil {
			return fmt.Errorf("invalid btrfs subvolume customization: %w", err)
		}
		if mountpoints[subvol.Mountpoint] {
			return fmt.Errorf("duplicate mountpoint %q in partitioning customizations", subvol.Mountpoint)
		}
		if slices.Contains(plainOnlyMountpoints, subvol.Mountpoint) {
			return fmt.Errorf("invalid mountpoint %q for btrfs subvolume", subvol.Mountpoint)
		}
		mountpoints[subvol.Mountpoint] = true
	}
	return nil
}

// CheckDiskMountpointsPolicy checks if the mountpoints under a [DiskCustomization] are allowed by the policy.
func CheckDiskMountpointsPolicy(partitioning *DiskCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	if partitioning == nil {
		return nil
	}

	// collect all mountpoints
	var mountpoints []string
	for _, part := range partitioning.Partitions {
		if part.Mountpoint != "" {
			mountpoints = append(mountpoints, part.Mountpoint)
		}
		for _, lv := range part.LogicalVolumes {
			if lv.Mountpoint != "" {
				mountpoints = append(mountpoints, lv.Mountpoint)
			}
		}
		for _, subvol := range part.Subvolumes {
			mountpoints = append(mountpoints, subvol.Mountpoint)
		}
	}

	var errs []error
	for _, mp := range mountpoints {
		if err := mountpointAllowList.Check(mp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while setting up custom mountpoints:\n%w", errors.Join(errs...))
	}

	return nil
}
