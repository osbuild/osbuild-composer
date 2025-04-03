package fedora

import (
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
)

func partitionTableLoader(t distro.ImageType) (*disk.PartitionTable, error) {
	return defs.PartitionTable(t, VersionReplacements())
}
