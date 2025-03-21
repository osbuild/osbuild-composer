package manifest

import (
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
)

// filesystemConfigStages generates either an org.osbuild.fstab stage or a
// collection of org.osbuild.systemd.unit.create stages for .mount and .swap
// units (and an org.osbuild.systemd stage to enable them) depending on the
// pipeline configuration.
func filesystemConfigStages(pt *disk.PartitionTable, mountUnits bool) ([]*osbuild.Stage, error) {
	if mountUnits {
		return osbuild.GenSystemdMountStages(pt)
	} else {
		opts, err := osbuild.NewFSTabStageOptions(pt)
		if err != nil {
			return nil, err
		}
		return []*osbuild.Stage{osbuild.NewFSTabStage(opts)}, nil
	}
}
