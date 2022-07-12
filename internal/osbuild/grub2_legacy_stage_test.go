package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrub2LegacyStage_Validation(t *testing.T) {

	options := GRUB2LegacyStageOptions{}

	err := options.validate()
	assert.Error(t, err)

	options.RootFS.Device = "/dev/sda"
	err = options.validate()
	assert.Error(t, err)

	prod := GRUB2Product{
		Name:    "Fedora",
		Nick:    "Foo",
		Version: "1",
	}
	options.Entries = MakeGrub2MenuEntries("id", "kernel", prod, false)
	err = options.validate()
	assert.Error(t, err)

	options.BIOS = &GRUB2BIOS{
		Platform: "i386-pc",
	}
	err = options.validate()
	assert.NoError(t, err)
}
