package disk

import (
	"math/rand"
	"reflect"

	"github.com/google/uuid"
)

// Swap defines the payload for a swap partition. It's similar to a
// [Filesystem] but with fewer fields. It is a [PayloadEntity] and also a
// [FSTabEntity].
type Swap struct {
	UUID  string
	Label string

	// The fourth field of fstab(5); fs_mntops
	FSTabOptions string
}

func init() {
	payloadEntityMap["swap"] = reflect.TypeOf(Swap{})
}

func (s *Swap) EntityName() string {
	return "swap"
}

func (s *Swap) Clone() Entity {
	if s == nil {
		return nil
	}

	return &Swap{
		UUID:         s.UUID,
		Label:        s.Label,
		FSTabOptions: s.FSTabOptions,
	}
}

// For swap, the fs_file entry in the fstab is always "none".
func (s *Swap) GetFSFile() string {
	return "none"
}

// For swap, the fs_vfstype entry in the fstab is always "swap".
func (s *Swap) GetFSType() string {
	return "swap"
}

func (s *Swap) GetFSSpec() FSSpec {
	if s == nil {
		return FSSpec{}
	}
	return FSSpec{
		UUID:  s.UUID,
		Label: s.Label,
	}
}

// For swap, the Freq and PassNo are always 0.
func (s *Swap) GetFSTabOptions() (FSTabOptions, error) {
	if s == nil {
		return FSTabOptions{}, nil
	}
	return FSTabOptions{
		MntOps: s.FSTabOptions,
		Freq:   0,
		PassNo: 0,
	}, nil
}

func (s *Swap) GenUUID(rng *rand.Rand) {
	if s.UUID == "" {
		s.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}
