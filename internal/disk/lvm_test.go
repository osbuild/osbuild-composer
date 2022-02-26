package disk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLVMVCreateMountpoint(t *testing.T) {

	assert := assert.New(t)

	vg := &LVMVolumeGroup{
		Name:        "root",
		Description: "root volume group",
	}

	entity, err := vg.CreateMountpoint("/", 0)
	assert.NoError(err)
	rootlv := entity.(*LVMLogicalVolume)
	assert.Equal("rootlv", rootlv.Name)

	_, err = vg.CreateMountpoint("/home_test", 0)
	assert.NoError(err)

	entity, err = vg.CreateMountpoint("/home/test", 0)
	assert.NoError(err)

	dedup := entity.(*LVMLogicalVolume)
	assert.Equal("home_testlv00", dedup.Name)

	// Lets collide it
	for i := 0; i < 98; i++ {
		_, err = vg.CreateMountpoint("/home/test", 0)
		assert.NoError(err)
	}

	_, err = vg.CreateMountpoint("/home/test", 0)
	assert.Error(err)
}
