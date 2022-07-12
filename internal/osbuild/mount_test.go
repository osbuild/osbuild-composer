package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMounts(t *testing.T) {
	assert := assert.New(t)

	{ // btrfs
		actual := NewBtrfsMount("btrfs", "/dev/sda1", "/mnt/btrfs")
		expected := &Mount{
			Name:   "btrfs",
			Type:   "org.osbuild.btrfs",
			Source: "/dev/sda1",
			Target: "/mnt/btrfs",
		}
		assert.Equal(expected, actual)
	}

	{ // ext4
		actual := NewExt4Mount("ext4", "/dev/sda2", "/mnt/ext4")
		expected := &Mount{
			Name:   "ext4",
			Type:   "org.osbuild.ext4",
			Source: "/dev/sda2",
			Target: "/mnt/ext4",
		}
		assert.Equal(expected, actual)
	}

	{ // fat
		actual := NewFATMount("fat", "/dev/sda3", "/mnt/fat")
		expected := &Mount{
			Name:   "fat",
			Type:   "org.osbuild.fat",
			Source: "/dev/sda3",
			Target: "/mnt/fat",
		}
		assert.Equal(expected, actual)
	}

	{ // xfs
		actual := NewXfsMount("xfs", "/dev/sda4", "/mnt/xfs")
		expected := &Mount{
			Name:   "xfs",
			Type:   "org.osbuild.xfs",
			Source: "/dev/sda4",
			Target: "/mnt/xfs",
		}
		assert.Equal(expected, actual)
	}
}
