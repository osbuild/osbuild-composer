package disk

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"
)

type Argon2id struct {
	Iterations  uint
	Memory      uint
	Parallelism uint
}
type LUKSContainer struct {
	Passphrase string
	UUID       string
	Cipher     string
	Label      string
	Subsystem  string
	SectorSize uint64

	// password-based key derivation function
	PBKDF Argon2id

	Payload Entity
}

func (lc *LUKSContainer) IsContainer() bool {
	return true
}

func (lc *LUKSContainer) GetItemCount() uint {
	if lc.Payload == nil {
		return 0
	}
	return 1
}

func (lc *LUKSContainer) GetChild(n uint) Entity {
	if n != 0 {
		panic(fmt.Sprintf("invalid child index for LUKSContainer: %d != 0", n))
	}
	return lc.Payload
}

func (lc *LUKSContainer) Clone() Entity {
	if lc == nil {
		return nil
	}

	return &LUKSContainer{
		Passphrase: lc.Passphrase,
		UUID:       lc.UUID,
		Cipher:     lc.Cipher,
		Label:      lc.Label,
		Subsystem:  lc.Subsystem,
		SectorSize: lc.SectorSize,
		PBKDF: Argon2id{
			Iterations:  lc.PBKDF.Iterations,
			Memory:      lc.PBKDF.Memory,
			Parallelism: lc.PBKDF.Parallelism,
		},
		Payload: lc.Payload,
	}
}

func (lc *LUKSContainer) GenUUID(rng *rand.Rand) {
	if lc == nil {
		return
	}

	if lc.UUID == "" {
		lc.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}
}

func (lc *LUKSContainer) MetadataSize() uint64 {
	if lc == nil {
		return 0
	}

	// 16 MiB is the default size for the LUKS2 header
	return 16 * 1024 * 1024
}
