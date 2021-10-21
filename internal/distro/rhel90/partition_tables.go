package rhel90

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
				Size:     2048,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 204800,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
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
				Size: 204800,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},
	distro.Ppc64leArchName: disk.PartitionTable{
		UUID: "0x14fc63d2",
		Type: "dos",
		Partitions: []disk.Partition{
			{
				Size:     8192,
				Type:     "41",
				Bootable: true,
			},
			{
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},
	distro.S390xArchName: disk.PartitionTable{
		UUID: "0x14fc63d2",
		Type: "dos",
		Partitions: []disk.Partition{
			{
				Bootable: true,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},
}

var ec2BasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     2048,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
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
				Size: 409600,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1048576,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
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
var edgeBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     2048, // 2MB
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 260096, // 127 MB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Filesystem: &disk.Filesystem{
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
				Size: 786432, // 384 MB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
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
				Size: 260096, // 127 MB
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Filesystem: &disk.Filesystem{
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
				Size: 786432, // 384 MB
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
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
