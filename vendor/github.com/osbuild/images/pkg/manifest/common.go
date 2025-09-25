package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
)

// filesystemConfigStages generates either an org.osbuild.fstab stage or a
// collection of org.osbuild.systemd.unit.create stages for .mount and .swap
// units (and an org.osbuild.systemd stage to enable them) depending on the
// pipeline configuration.
func filesystemConfigStages(pt *disk.PartitionTable, mountConfiguration osbuild.MountConfiguration) ([]*osbuild.Stage, error) {
	switch mountConfiguration {
	case osbuild.MOUNT_CONFIGURATION_UNITS:
		return osbuild.GenSystemdMountStages(pt)
	case osbuild.MOUNT_CONFIGURATION_FSTAB:
		opts, err := osbuild.NewFSTabStageOptions(pt)
		if err != nil {
			return nil, err
		}
		return []*osbuild.Stage{osbuild.NewFSTabStage(opts)}, nil
	case osbuild.MOUNT_CONFIGURATION_NONE:
		return []*osbuild.Stage{}, nil
	default:
		return nil, fmt.Errorf("Unexpected mount configuration %d", mountConfiguration)
	}
}
