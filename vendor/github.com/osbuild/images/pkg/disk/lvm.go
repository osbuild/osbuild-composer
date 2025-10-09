package disk

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
)

// Default physical extent size in bytes: logical volumes
// created inside the VG will be aligned to this.
const LVMDefaultExtentSize = 4 * datasizes.MebiByte

type LVMVolumeGroup struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	LogicalVolumes []LVMLogicalVolume `json:"logical_volumes,omitempty" yaml:"logical_volumes,omitempty"`
}

var _ = MountpointCreator(&LVMVolumeGroup{})

func init() {
	payloadEntityMap["lvm"] = reflect.TypeOf(LVMVolumeGroup{})
}

func (vg *LVMVolumeGroup) EntityName() string {
	return "lvm"
}

func (vg *LVMVolumeGroup) Clone() Entity {
	if vg == nil {
		return nil
	}

	clone := &LVMVolumeGroup{
		Name:           vg.Name,
		Description:    vg.Description,
		LogicalVolumes: make([]LVMLogicalVolume, len(vg.LogicalVolumes)),
	}

	for idx, lv := range vg.LogicalVolumes {
		ent := lv.Clone()

		// lv.Clone() will return nil only if the logical volume is nil
		if ent == nil {
			panic(fmt.Sprintf("logical volume %d in a LVM volume group is nil; this is a programming error", idx))
		}

		lv, cloneOk := ent.(*LVMLogicalVolume)
		if !cloneOk {
			panic("LVMLogicalVolume.Clone() returned an Entity that cannot be converted to *LVMLogicalVolume; this is a programming error")
		}

		clone.LogicalVolumes[idx] = *lv
	}

	return clone
}

func (vg *LVMVolumeGroup) GetItemCount() uint {
	if vg == nil {
		return 0
	}
	return uint(len(vg.LogicalVolumes))
}

func (vg *LVMVolumeGroup) GetChild(n uint) Entity {
	if vg == nil {
		panic("LVMVolumeGroup.GetChild: nil entity")
	}
	return &vg.LogicalVolumes[n]
}

func (vg *LVMVolumeGroup) CreateMountpoint(mountpoint, defaultFs string, size datasizes.Size) (Entity, error) {
	if defaultFs == "btrfs" {
		return nil, fmt.Errorf("btrfs under lvm is not supported")
	}

	filesystem := Filesystem{
		Type:         defaultFs,
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}

	// leave lv name empty to autogenerate based on mountpoint
	return vg.CreateLogicalVolume("", size, &filesystem)
}

// genLVName generates a valid logical volume name from a mountpoint or base
// that does not conflict with existing ones.
func (vg *LVMVolumeGroup) genLVName(base string) (string, error) {
	names := make(map[string]bool, len(vg.LogicalVolumes))
	for _, lv := range vg.LogicalVolumes {
		names[lv.Name] = true
	}

	base = lvname(base) // if the mountpoint is used (i.e. if the base contains /), sanitize it and append 'lv'

	// Make sure that we don't collide with an existing volume, e.g.
	// 'home/test' and /home_test would collide.
	return genUniqueString(base, names)
}

// CreateLogicalVolume creates a new logical volume on the volume group. If a
// name is not provided, a valid one is generated based on the payload
// mountpoint. If a name is provided, it is used directly without validating.
func (vg *LVMVolumeGroup) CreateLogicalVolume(lvName string, size datasizes.Size, payload Entity) (*LVMLogicalVolume, error) {
	if vg == nil {
		panic("LVMVolumeGroup.CreateLogicalVolume: nil entity")
	}

	if lvName == "" {
		// generate a name based on the payload's mountpoint
		switch ent := payload.(type) {
		case Mountable:
			lvName = ent.GetMountpoint()
		case *Swap:
			lvName = "swap"
		default:
			return nil, fmt.Errorf("could not create logical volume: no name provided and payload %T is not mountable or swap", payload)
		}
		autoName, err := vg.genLVName(lvName)
		if err != nil {
			return nil, err
		}
		lvName = autoName
	}

	lv := LVMLogicalVolume{
		Name:    lvName,
		Size:    vg.AlignUp(size),
		Payload: payload,
	}

	vg.LogicalVolumes = append(vg.LogicalVolumes, lv)

	return &vg.LogicalVolumes[len(vg.LogicalVolumes)-1], nil
}

func alignUp(size datasizes.Size) datasizes.Size {
	if size%LVMDefaultExtentSize != 0 {
		size += LVMDefaultExtentSize - size%LVMDefaultExtentSize
	}

	return size
}

func (vg *LVMVolumeGroup) AlignUp(size datasizes.Size) datasizes.Size {
	return alignUp(size)
}

func (vg *LVMVolumeGroup) MetadataSize() datasizes.Size {
	if vg == nil {
		return 0
	}

	// LVM2 allows for a lot of customizations that will affect the size
	// of the metadata and its location and thus the start of the physical
	// extent. For now we assume the default which results in a start of
	// the physical extent 1 MiB
	return 1 * datasizes.MiB
}

func (vg *LVMVolumeGroup) minSize(size datasizes.Size) datasizes.Size {
	var lvsum datasizes.Size
	for _, lv := range vg.LogicalVolumes {
		lvsum += lv.Size
	}
	minSize := lvsum + vg.MetadataSize()

	if minSize > size {
		size = minSize
	}

	return vg.AlignUp(size)
}

func (vg *LVMVolumeGroup) UnmarshalJSON(data []byte) error {
	type alias LVMVolumeGroup
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*vg = LVMVolumeGroup(tmp)
	return nil
}

type LVMLogicalVolume struct {
	Name    string         `json:"name,omitempty" yaml:"name,omitempty"`
	Size    datasizes.Size `json:"size,omitempty" yaml:"size,omitempty"`
	Payload Entity         `json:"payload,omitempty" yaml:"payload,omitempty"`
}

func (lv *LVMLogicalVolume) Clone() Entity {
	if lv == nil {
		return nil
	}
	return &LVMLogicalVolume{
		Name:    lv.Name,
		Size:    lv.Size,
		Payload: lv.Payload.Clone(),
	}
}

func (lv *LVMLogicalVolume) GetItemCount() uint {
	if lv == nil || lv.Payload == nil {
		return 0
	}
	return 1
}

func (lv *LVMLogicalVolume) GetChild(n uint) Entity {
	if n != 0 || lv == nil {
		panic(fmt.Sprintf("invalid child index for LVMLogicalVolume: %d != 0", n))
	}
	return lv.Payload
}

func (lv *LVMLogicalVolume) GetSize() datasizes.Size {
	if lv == nil {
		return 0
	}
	return lv.Size
}

func (lv *LVMLogicalVolume) EnsureSize(s datasizes.Size) bool {
	if lv == nil {
		panic("LVMLogicalVolume.EnsureSize: nil entity")
	}
	if s > lv.Size {
		lv.Size = alignUp(s)
		return true
	}
	return false
}

// lvname returns a name for a logical volume based on the mountpoint.
func lvname(path string) string {
	if path == "/" {
		return "rootlv"
	}

	path = strings.TrimLeft(path, "/")
	return strings.ReplaceAll(path, "/", "_") + "lv"
}

func (lv *LVMLogicalVolume) UnmarshalJSON(data []byte) (err error) {
	// keep in sync with lvm.go,partition.go,luks.go
	type alias LVMLogicalVolume
	var withoutPayload struct {
		alias
		Payload     json.RawMessage `json:"payload" yaml:"payload"`
		PayloadType string          `json:"payload_type" yaml:"payload_type"`
	}
	if err := jsonUnmarshalStrict(data, &withoutPayload); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", data, err)
	}
	*lv = LVMLogicalVolume(withoutPayload.alias)

	lv.Payload, err = unmarshalJSONPayload(data)
	return err
}

func (lv *LVMLogicalVolume) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(lv, unmarshal)
}
