package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLVM2CreateStageValidation(t *testing.T) {
	assert := assert.New(t)

	okOptions := LVM2CreateStageOptions{
		Volumes: []LogicalVolume{
			{
				Name: "a_volume_name",
				Size: "",
			},
			{
				Name: "good-volume.name",
				Size: "10G",
			},
			{
				Name: "99-luft+volumes",
				Size: "10737418240",
			},
			{
				Name: "++",
				Size: "1337",
			},
			{
				Name: "_",
				Size: "0",
			},
		},
	}
	assert.NoError(okOptions.validate())

	badVolumes := []LogicalVolume{
		{
			Name: "!bad-bad-volume-name",
			Size: "1337",
		},
		{
			Name: "even worse",
		},
		{
			Name: "-",
		},
	}

	for _, vol := range badVolumes {
		options := LVM2CreateStageOptions{
			Volumes: []LogicalVolume{vol},
		}
		assert.Error(options.validate(), vol.Name)
	}

	empty := LVM2CreateStageOptions{}
	assert.Error(empty.validate())
}
