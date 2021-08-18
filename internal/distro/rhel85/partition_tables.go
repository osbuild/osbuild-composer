package rhel85

import (
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

var defaultArches = map[string]disk.PartitionTable{
	distro.X86_64ArchName:  gptPartitionTable,
	distro.Aarch64ArchName: gptPartitionTable,
	distro.Ppc64leArchName: dosPartitionTable,
	distro.S390xArchName:   dosPartitionTable,
}

var ec2ValidArches = map[string]disk.PartitionTable{
	distro.X86_64ArchName:  gptPartitionTable,
	distro.Aarch64ArchName: gptPartitionTable,
}

var gptPartitionTable = disk.PartitionTable{
	UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type:       "gpt",
	Partitions: []disk.Partition{},
}

var dosPartitionTable = disk.PartitionTable{
	UUID:       "0x14fc63d2",
	Type:       "dos",
	Partitions: []disk.Partition{},
}

func getBasePartitionTable(arch distro.Arch, validArches ValidArches) disk.PartitionTable {
	archName := arch.Name()
	partitionTable, ok := validArches[archName]

	if !ok {
		panic("unknown arch: " + archName)
	}

	return partitionTable
}
