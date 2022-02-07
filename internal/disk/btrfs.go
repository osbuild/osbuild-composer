package disk

import (
	"fmt"
	"strings"
)

type Btrfs struct {
	UUID       string
	Label      string
	Mountpoint string
	Subvolumes []BtrfsSubvolume
}

func (b *Btrfs) IsContainer() bool {
	return true
}

func (b *Btrfs) GetItemCount() uint {
	return uint(len(b.Subvolumes))
}

func (b *Btrfs) GetChild(n uint) Entity {
	return &b.Subvolumes[n]
}
func (b *Btrfs) CreateVolume(mountpoint string, size uint64) (Entity, error) {
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
	}

	b.Subvolumes = append(b.Subvolumes, subvolume)
	return &b.Subvolumes[len(b.Subvolumes)-1], nil
}

type BtrfsSubvolume struct {
	Name       string
	Size       uint64
	Mountpoint string
	GroupID    uint64

	MntOps string

	// UUID of the parent volume
	UUID string
}

func (subvol *BtrfsSubvolume) IsContainer() bool {
	return false
}

func (bs *BtrfsSubvolume) GetSize() uint64 {
	return bs.Size
}

func (bs *BtrfsSubvolume) EnsureSize(s uint64) bool {
	if s > bs.Size {
		bs.Size = s
		return true
	}
	return false
}

func (bs *BtrfsSubvolume) GetMountpoint() string {
	return bs.Mountpoint
}

func (bs *BtrfsSubvolume) GetFSType() string {
	return "btrfs"
}

func (bs *BtrfsSubvolume) GetFSSpec() FSSpec {
	return FSSpec{
		UUID:  bs.UUID,
		Label: bs.Name,
	}
}

func (bs *BtrfsSubvolume) GetFSTabOptions() FSTabOptions {
	ops := strings.Join([]string{bs.MntOps, fmt.Sprintf("subvol=%s", bs.Name)}, ",")

	return FSTabOptions{
		MntOps: ops,
		Freq:   0,
		PassNo: 0,
	}
}
