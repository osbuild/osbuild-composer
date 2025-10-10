package disk

import (
	"fmt"
	"math/rand"
	"reflect"

	"github.com/google/uuid"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
)

const DefaultBtrfsCompression = "zstd:1"

type Btrfs struct {
	UUID       string           `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Label      string           `json:"label,omitempty" yaml:"label,omitempty"`
	Mountpoint string           `json:"mountpoint,omitempty" yaml:"mountpoint,omitempty"`
	Subvolumes []BtrfsSubvolume `json:"subvolumes,omitempty" yaml:"subvolumes,omitempty"`
}

var _ = MountpointCreator(&Btrfs{})

func init() {
	payloadEntityMap["btrfs"] = reflect.TypeOf(Btrfs{})
}

func (b *Btrfs) EntityName() string {
	return "btrfs"
}

func (b *Btrfs) Clone() Entity {
	if b == nil {
		return nil
	}

	clone := &Btrfs{
		UUID:       b.UUID,
		Label:      b.Label,
		Mountpoint: b.Mountpoint,
		Subvolumes: make([]BtrfsSubvolume, len(b.Subvolumes)),
	}

	for idx, subvol := range b.Subvolumes {
		entClone := subvol.Clone()
		svClone, cloneOk := entClone.(*BtrfsSubvolume)
		if !cloneOk {
			panic("BtrfsSubvolume.Clone() returned an Entity that cannot be converted to *BtrfsSubvolume; this is a programming error")
		}
		clone.Subvolumes[idx] = *svClone
	}

	return clone
}

func (b *Btrfs) GetItemCount() uint {
	return uint(len(b.Subvolumes))
}

func (b *Btrfs) GetChild(n uint) Entity {
	return &b.Subvolumes[n]
}
func (b *Btrfs) CreateMountpoint(mountpoint, defaultFs string, size datasizes.Size) (Entity, error) {
	if defaultFs != "btrfs" {
		return nil, fmt.Errorf("only btrfs mountpoints are supported with btrfs subvolumes not %q", defaultFs)
	}

	name := mountpoint
	if name == "/" {
		name = "root"
	}
	subvolume := BtrfsSubvolume{
		Size:       size,
		Mountpoint: mountpoint,
		GroupID:    0,
		UUID:       b.UUID, // subvolumes inherit UUID of main volume
		Name:       name,
		Compress:   DefaultBtrfsCompression,
	}

	b.Subvolumes = append(b.Subvolumes, subvolume)
	return &b.Subvolumes[len(b.Subvolumes)-1], nil
}

func (b *Btrfs) AlignUp(size datasizes.Size) datasizes.Size {
	return size // No extra alignment necessary for subvolumes
}

func (b *Btrfs) GenUUID(rng *rand.Rand) {
	if b.UUID == "" {
		b.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}

	for i := range b.Subvolumes {
		b.Subvolumes[i].UUID = b.UUID
	}
}

func (b *Btrfs) MetadataSize() datasizes.Size {
	return 0
}

func (b *Btrfs) minSize(size datasizes.Size) datasizes.Size {
	var subvolsum datasizes.Size
	for _, sv := range b.Subvolumes {
		subvolsum += sv.Size
	}
	minSize := subvolsum + b.MetadataSize()

	if minSize > size {
		size = minSize
	}

	return b.AlignUp(size)
}

type BtrfsSubvolume struct {
	Name       string         `json:"name" yaml:"name"`
	Size       datasizes.Size `json:"size" yaml:"size"`
	Mountpoint string         `json:"mountpoint,omitempty" yaml:"mountpoint,omitempty"`
	GroupID    uint64         `json:"group_id,omitempty" yaml:"group_id,omitempty"`
	Compress   string         `json:"compress,omitempty" yaml:"compress,omitempty"`
	ReadOnly   bool           `json:"read_only,omitempty" yaml:"read_only,omitempty"`

	// UUID of the parent volume
	UUID string `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

func (sv *BtrfsSubvolume) UnmarshalJSON(data []byte) (err error) {
	type aliasStruct BtrfsSubvolume
	var alias aliasStruct
	if err := jsonUnmarshalStrict(data, &alias); err != nil {
		return fmt.Errorf("cannot unmarshal %q: %w", data, err)
	}
	*sv = BtrfsSubvolume(alias)
	return err
}

func (sv *BtrfsSubvolume) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(sv, unmarshal)
}

func (bs *BtrfsSubvolume) Clone() Entity {
	if bs == nil {
		return nil
	}

	return &BtrfsSubvolume{
		Name:       bs.Name,
		Size:       bs.Size,
		Mountpoint: bs.Mountpoint,
		GroupID:    bs.GroupID,
		Compress:   bs.Compress,
		UUID:       bs.UUID,
	}
}

func (bs *BtrfsSubvolume) GetSize() datasizes.Size {
	if bs == nil {
		return 0
	}
	return bs.Size
}

func (bs *BtrfsSubvolume) EnsureSize(s datasizes.Size) bool {
	if s > bs.Size {
		bs.Size = s
		return true
	}
	return false
}

func (bs *BtrfsSubvolume) GetMountpoint() string {
	if bs == nil {
		return ""
	}
	return bs.Mountpoint
}

func (bs *BtrfsSubvolume) GetFSFile() string {
	return bs.GetMountpoint()
}

func (bs *BtrfsSubvolume) GetFSType() string {
	return "btrfs"
}

func (bs *BtrfsSubvolume) GetFSSpec() FSSpec {
	if bs == nil {
		return FSSpec{}
	}
	return FSSpec{
		UUID:  bs.UUID,
		Label: bs.Name,
	}
}

func (bs *BtrfsSubvolume) GetFSTabOptions() (FSTabOptions, error) {
	if bs == nil {
		return FSTabOptions{}, nil
	}

	if bs.Name == "" {
		return FSTabOptions{}, fmt.Errorf("internal error: BtrfsSubvolume.GetFSTabOptions() for %+v called without a name", bs)
	}
	ops := fmt.Sprintf("subvol=%s", bs.Name)
	if bs.Compress != "" {
		ops += fmt.Sprintf(",compress=%s", bs.Compress)
	}
	if bs.ReadOnly {
		ops += ",ro"
	}
	return FSTabOptions{
		MntOps: ops,
		Freq:   0,
		PassNo: 0,
	}, nil
}
