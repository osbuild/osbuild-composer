package osbuild

import (
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/stretchr/testify/assert"
)

func TestGenImageKernelOptions(t *testing.T) {
	assert := assert.New(t)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))

	luks_lvm := testPartitionTables["luks+lvm"]

	pt, err := disk.NewPartitionTable(&luks_lvm, []blueprint.FilesystemCustomization{}, 0, false, rng)
	assert.NoError(err)

	var uuid string

	findLuksUUID := func(e disk.Entity, path []disk.Entity) error {
		switch ent := e.(type) {
		case *disk.LUKSContainer:
			uuid = ent.UUID
		}

		return nil
	}
	_ = pt.ForEachEntity(findLuksUUID)

	assert.NotEmpty(uuid, "Could not find LUKS container")
	cmdline := GenImageKernelOptions(pt)

	assert.Subset(cmdline, []string{"luks.uuid=" + uuid})
}
