package fedora

import (
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

var defaultBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 524288000, // 500 MB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size: 2147483648, // 2GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},
	distro.Aarch64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size: 209715200, // 200 MB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 524288000, // 500 MB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size: 2147483648, // 2GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},
}

var iotBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 525336576, // 500 MiB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "umask=0077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1073741824, // 1 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2693791744, // 2.5 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
	distro.Aarch64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 525336576, // 500 MiB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "umask=0077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1073741824, // 1 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2693791744, // 2.5 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "ext4",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
		},
	},
}
