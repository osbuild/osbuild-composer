package disk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLVMVGCreateVolume(t *testing.T) {

	assert := assert.New(t)

	vg := &LVMVolumeGroup{
		Name:        "root",
		Description: "root volume group",
	}

	entity, err := vg.CreateVolume("/", 0)
	assert.NoError(err)
	rootlv := entity.(*LVMLogicalVolume)
	assert.Equal("rootlv", rootlv.Name)

	_, err = vg.CreateVolume("/home_test", 0)
	assert.NoError(err)

	entity, err = vg.CreateVolume("/home/test", 0)
	assert.NoError(err)

	dedup := entity.(*LVMLogicalVolume)
	assert.Equal("home_testlv00", dedup.Name)

	// Lets collide it
	for i := 0; i < 98; i++ {
		_, err = vg.CreateVolume("/home/test", 0)
		assert.NoError(err)
	}

	_, err = vg.CreateVolume("/home/test", 0)
	assert.Error(err)
}
