package disk

import osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"

// Filesystem related functions
type Filesystem struct {
	Type string
	// ID of the filesystem, vfat doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID       string
	Label      string
	Mountpoint string
	// The fourth field of fstab(5); fs_mntops
	FSTabOptions string
	// The fifth field of fstab(5); fs_freq
	FSTabFreq uint64
	// The sixth field of fstab(5); fs_passno
	FSTabPassNo uint64
}

func (fs *Filesystem) IsContainer() bool {
	return false
}

// Clone the filesystem structure
func (fs *Filesystem) Clone() Entity {
	if fs == nil {
		return nil
	}

	return &Filesystem{
		Type:         fs.Type,
		UUID:         fs.UUID,
		Label:        fs.Label,
		Mountpoint:   fs.Mountpoint,
		FSTabOptions: fs.FSTabOptions,
		FSTabFreq:    fs.FSTabFreq,
		FSTabPassNo:  fs.FSTabPassNo,
	}
}

// Converts Filesystem to osbuild.QEMUFilesystem that encodes the same fs.
func (fs *Filesystem) QEMUFilesystem() osbuild.QEMUFilesystem {
	return osbuild.QEMUFilesystem{
		Type:       fs.Type,
		UUID:       fs.UUID,
		Label:      fs.Label,
		Mountpoint: fs.Mountpoint,
	}
}

func (fs *Filesystem) GetMountpoint() string {
	return fs.Mountpoint
}

func (fs *Filesystem) GetFSType() string {
	return fs.Type
}

func (fs *Filesystem) GetFSSpec() FSSpec {
	return FSSpec{
		UUID:  fs.UUID,
		Label: fs.Label,
	}
}

func (fs *Filesystem) GetFSTabOptions() FSTabOptions {
	return FSTabOptions{
		MntOps: fs.FSTabOptions,
		Freq:   fs.FSTabFreq,
		PassNo: fs.FSTabPassNo,
	}
}
