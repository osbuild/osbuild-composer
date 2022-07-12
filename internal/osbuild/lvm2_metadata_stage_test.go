package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLVM2MetadataStageValidation(t *testing.T) {
	assert := assert.New(t)

	okOptions := []LVM2MetadataStageOptions{
		{
			VGName:       "a_volume_name",
			CreationTime: "0",
		},
		{
			VGName:       "good-volume.name",
			CreationTime: "1629282647",
		},
		{
			VGName:       "99-luft+volumes",
			CreationTime: "2147483648",
		},
		{
			VGName:       "++",
			CreationTime: "42",
		},
		{
			VGName:       "_",
			CreationTime: "4294967297",
		},
	}
	for _, o := range okOptions {
		assert.NoError(o.validate(), o)
	}

	badOptions := []LVM2MetadataStageOptions{
		{
			VGName:       "ok-name-bad-time",
			CreationTime: "-10",
		},
		{
			VGName:       "!bad-name",
			CreationTime: "1629282647",
		},
		{
			VGName:       "worse.time",
			CreationTime: "TIME",
		},
	}
	for _, o := range badOptions {
		assert.Error(o.validate(), o)
	}
}
