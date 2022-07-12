package osbuild

import (
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/stretchr/testify/assert"
)

func TestGenDeviceCreationStages(t *testing.T) {
	assert := assert.New(t)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))

	luks_lvm := testPartitionTables["luks+lvm"]

	pt, err := disk.NewPartitionTable(&luks_lvm, []blueprint.FilesystemCustomization{}, 0, false, rng)
	assert.NoError(err)

	stages := GenDeviceCreationStages(pt, "image.raw")

	// we should have two stages
	assert.Equal(len(stages), 2)

	// first one should be a "org.osbuild.luks2.format"
	luks := stages[0]
	assert.Equal(luks.Type, "org.osbuild.luks2.format")

	// it needs to have one device
	assert.Equal(len(luks.Devices), 1)

	// the device should be called `device`
	device, ok := luks.Devices["device"]
	assert.True(ok, "Need device called `device`")

	// device should be a loopback device
	assert.Equal(device.Type, "org.osbuild.loopback")

	lvm := stages[1]
	assert.Equal(lvm.Type, "org.osbuild.lvm2.create")
	lvmOptions, ok := lvm.Options.(*LVM2CreateStageOptions)
	assert.True(ok, "Need LVM2CreateStageOptions for org.osbuild.lvm2.create")

	// LVM should have two volumes
	assert.Equal(len(lvmOptions.Volumes), 2)
	rootlv := lvmOptions.Volumes[0]
	assert.Equal(rootlv.Name, "rootlv")

	homelv := lvmOptions.Volumes[1]
	assert.Equal(homelv.Name, "homelv")

	// it needs to have two(!) devices, the loopback and the luks
	assert.Equal(len(lvm.Devices), 2)

	// this is the target one, which should be the luks one
	device, ok = lvm.Devices["device"]
	assert.True(ok, "Need device called `device`")
	assert.Equal(device.Type, "org.osbuild.luks2")
	assert.NotEmpty(device.Parent, "Need a parent device for LUKS on loopback")

	luksOptions, ok := device.Options.(*LUKS2DeviceOptions)
	assert.True(ok, "Need LUKS2DeviceOptions for luks device")
	assert.Equal(luksOptions.Passphrase, "osbuild")

	parent, ok := lvm.Devices[device.Parent]
	assert.True(ok, "Need device called `device`")
	assert.Equal(parent.Type, "org.osbuild.loopback")

}

func TestGenDeviceFinishStages(t *testing.T) {
	assert := assert.New(t)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))

	luks_lvm := testPartitionTables["luks+lvm"]

	pt, err := disk.NewPartitionTable(&luks_lvm, []blueprint.FilesystemCustomization{}, 0, false, rng)
	assert.NoError(err)

	stages := GenDeviceFinishStages(pt, "image.raw")

	// we should have one stage
	assert.Equal(1, len(stages))

	// it should be a "org.osbuild.lvm2.metadata"
	lvm := stages[0]
	assert.Equal("org.osbuild.lvm2.metadata", lvm.Type)

	// it should have two devices
	assert.Equal(2, len(lvm.Devices))

	// this is the target one, which should be the luks one
	device, ok := lvm.Devices["device"]
	assert.True(ok, "Need device called `device`")
	assert.Equal("org.osbuild.luks2", device.Type)
	assert.NotEmpty(device.Parent, "Need a parent device for LUKS on loopback")

	luksOptions, ok := device.Options.(*LUKS2DeviceOptions)
	assert.True(ok, "Need LUKS2DeviceOptions for luks device")
	assert.Equal("osbuild", luksOptions.Passphrase)

	parent, ok := lvm.Devices[device.Parent]
	assert.True(ok, "Need device called `device`")
	assert.Equal("org.osbuild.loopback", parent.Type)

	opts, ok := lvm.Options.(*LVM2MetadataStageOptions)
	assert.True(ok, "Need LVM2MetadataStageOptions for org.osbuild.lvm2.metadata")
	assert.Equal("root", opts.VGName)
}
