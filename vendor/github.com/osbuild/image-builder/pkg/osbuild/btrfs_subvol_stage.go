package osbuild

type BtrfsSubVolOptions struct {
	Subvolumes []BtrfsSubVol `json:"subvolumes"`
}

type BtrfsSubVol struct {
	Name string `json:"name"`
}

func (BtrfsSubVolOptions) isStageOptions() {}

func NewBtrfsSubVol(options *BtrfsSubVolOptions, devices *map[string]Device, mounts *[]Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.btrfs.subvol",
		Options: options,
		Devices: *devices,
		Mounts:  *mounts,
	}
}
