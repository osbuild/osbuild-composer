package rhel85

import (
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

const (
	// For historical reasons the 8.5/9.0 beta images had some
	// extra padding at the end. The reason was to leave space
	// for the secondary GPT header, but it was too much and
	// also done for MBR. Since image definitions are frozen,
	// we keep this extra padding around.
	ExtraPaddingGPT = uint64(34304)
	ExtraPaddingMBR = uint64(51200)
)

var defaultBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size:     1048576,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 104857600,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size: 104857600,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
		UUID:         "0x14fc63d2",
		Type:         "dos",
		ExtraPadding: ExtraPaddingMBR,
		Partitions: []disk.Partition{
			{
				Size:     4194304,
				Type:     "41",
				Bootable: true,
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Payload: &disk.Filesystem{
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
		UUID:         "0x14fc63d2",
		Type:         "dos",
		ExtraPadding: ExtraPaddingMBR,
		Partitions: []disk.Partition{
			{
				Size:     2 * 1024 * 1024 * 1024, // 2 GiB
				Bootable: true,
				Payload: &disk.Filesystem{
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
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size:     1048576,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size: 209715200,
				Type: disk.EFISystemPartitionGUID,
				UUID: disk.EFISystemPartitionUUID,
				Payload: &disk.Filesystem{
					Type:         "vfat",
					UUID:         disk.EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 536870912,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size:     1048576,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 133169152,
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
				Size: 402653184,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
		UUID:         "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type:         "gpt",
		ExtraPadding: ExtraPaddingGPT,
		Partitions: []disk.Partition{
			{
				Size: 133169152,
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
				Size: 402653184,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    1,
					FSTabPassNo:  1,
				},
			},
			{
				Size: 2 * 1024 * 1024 * 1024, // 2 GiB
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Payload: &disk.Filesystem{
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
