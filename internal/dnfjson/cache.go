package dnfjson

import (
	"io/fs"
	"os"
	"path/filepath"
)

type rpmCache struct {
	// root path for the cache
	root string

	// max cache size
	maxSize uint64
}

func newRPMCache(path string, maxSize uint64) *rpmCache {
	return &rpmCache{
		root:    path,
		maxSize: maxSize,
	}
}

func (r *rpmCache) size() (uint64, error) {
	var size uint64
	sizer := func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		size += uint64(info.Size())
		return nil
	}
	err := filepath.Walk(r.root, sizer)
	return size, err
}

func (r *rpmCache) clean() error {
	curSize, err := r.size()
	if err != nil {
		return err
	}
	if curSize > r.maxSize {
		return os.RemoveAll(r.root)
	}
	return nil
}
