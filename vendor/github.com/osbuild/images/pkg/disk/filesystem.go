package disk

import (
	"math/rand"
	"reflect"

	"github.com/google/uuid"
)

// Filesystem related functions
type Filesystem struct {
	Type string `json:"type"`

	// ID of the filesystem, vfat doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID       string `json:"uuid,omitempty"`
	Label      string `json:"label,omitempty"`
	Mountpoint string `json:"mountpoint,omitempty"`
	// The fourth field of fstab(5); fs_mntops
	FSTabOptions string `json:"fstab_options,omitempty"`
	// The fifth field of fstab(5); fs_freq
	FSTabFreq uint64 `json:"fstab_freq,omitempty"`
	// The sixth field of fstab(5); fs_passno
	FSTabPassNo uint64 `json:"fstab_passno,omitempty"`
}

func init() {
	payloadEntityMap["filesystem"] = reflect.TypeOf(Filesystem{})
}

func (fs *Filesystem) EntityName() string {
	return "filesystem"
}

// Clone the filesystem structure
func (fs *Filesystem) Clone() Entity {
	if fs == nil {
		return nil
	}

	return &Filesystem{
		Type:         fs.Type,
		UUID:         fs.UUID,
		Label:        fs.Label,
		Mountpoint:   fs.Mountpoint,
		FSTabOptions: fs.FSTabOptions,
		FSTabFreq:    fs.FSTabFreq,
		FSTabPassNo:  fs.FSTabPassNo,
	}
}

func (fs *Filesystem) GetMountpoint() string {
	if fs == nil {
		return ""
	}
	return fs.Mountpoint
}

func (fs *Filesystem) GetFSFile() string {
	return fs.GetMountpoint()
}

func (fs *Filesystem) GetFSType() string {
	if fs == nil {
		return ""
	}
	return fs.Type
}

func (fs *Filesystem) GetFSSpec() FSSpec {
	if fs == nil {
		return FSSpec{}
	}
	return FSSpec{
		UUID:  fs.UUID,
		Label: fs.Label,
	}
}

func (fs *Filesystem) GetFSTabOptions() (FSTabOptions, error) {
	if fs == nil {
		return FSTabOptions{}, nil
	}
	return FSTabOptions{
		MntOps: fs.FSTabOptions,
		Freq:   fs.FSTabFreq,
		PassNo: fs.FSTabPassNo,
	}, nil
}

func (fs *Filesystem) GenUUID(rng *rand.Rand) {
	if fs.Type == "vfat" && fs.UUID == "" {
		// vfat has no uuids, it has "serial numbers" (volume IDs)
		fs.UUID = NewVolIDFromRand(rng)
		return
	}
	if fs.UUID == "" {
		fs.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}
