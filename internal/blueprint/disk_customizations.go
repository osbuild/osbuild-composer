package blueprint

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/pkg/datasizes"
)

type DiskCustomization struct {
	Type       string
	MinSize    uint64
	Partitions []PartitionCustomization
}

type diskCustomizationMarshaler struct {
	MinSize    datasizes.Size           `json:"minsize,omitempty" toml:"minsize,omitempty"`
	Partitions []PartitionCustomization `json:"partitions,omitempty" toml:"partitions,omitempty"`
}

func (dc *DiskCustomization) UnmarshalJSON(data []byte) error {
	var dcm diskCustomizationMarshaler
	if err := json.Unmarshal(data, &dcm); err != nil {
		return err
	}
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
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	Label      string `json:"label,omitempty" toml:"label,omitempty"`
	FSType     string `json:"fs_type,omitempty" toml:"fs_type,omitempty"`
}

// An LVM volume group with one or more logical volumes.
type VGCustomization struct {
	// Volume group name (optional, default will be automatically generated).
	Name           string            `json:"name,omitempty" toml:"name,omitempty"`
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
	Subvolumes []BtrfsSubvolumeCustomization `json:"subvolumes,omitempty" toml:"subvolumes,omitempty"`
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
		Type    string `json:"type"`
		MinSize any    `json:"minsize"`
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
		// Type and minsize are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type    string `json:"type"`
		MinSize any    `json:"minsize"`
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
		// Type and minsize are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type    string `json:"type"`
		MinSize any    `json:"minsize"`
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
		// Type and minsize are handled by the caller. These are added here to
		// satisfy "DisallowUnknownFields" when decoding.
		Type    string `json:"type"`
		MinSize any    `json:"minsize"`
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

func unmarshalTOMLviaJSON(u json.Unmarshaler, data any) error {
	// This is the most efficient way to reuse code when unmarshaling
	// structs in toml, it leaks json errors which is a bit sad but
	// because the toml unmarshaler gives us not "[]byte" but an
	// already pre-processed "any" we cannot just unmarshal into our
	// "fooMarshaling" struct and reuse the result so we resort to
	// this workaround (but toml will go away long term anyway).
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error unmarshaling TOML data %v: %w", data, err)
	}
	if err := u.UnmarshalJSON(dataJSON); err != nil {
		return fmt.Errorf("error decoding TOML %v: %w", data, err)
	}
	return nil
}

// decodeSize takes an integer or string representing a data size (with a data
// suffix) and returns the uint64 representation.
func decodeSize(size any) (uint64, error) {
	switch s := size.(type) {
	case string:
		return datasizes.Parse(s)
	case int64:
		if s < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		return uint64(s), nil
	case float64:
		if s < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		// TODO: emit warning of possible truncation?
		return uint64(s), nil
	case uint64:
		return s, nil
	default:
		return 0, fmt.Errorf("failed to convert value \"%v\" to number", size)
	}
}
