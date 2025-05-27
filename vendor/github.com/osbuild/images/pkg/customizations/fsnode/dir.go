package fsnode

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/osbuild/images/internal/common"
)

type Directory struct {
	baseFsNode
	ensureParentDirs bool
}

func (d *Directory) EnsureParentDirs() bool {
	if d == nil {
		return false
	}
	return d.ensureParentDirs
}

func (d *Directory) UnmarshalJSON(data []byte) error {
	var v struct {
		baseFsNodeJSON
		EnsureParentDirs bool `json:"ensure_parent_dirs"`
	}
	dec := json.NewDecoder(bytes.NewBuffer(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	d.baseFsNode.baseFsNodeJSON = v.baseFsNodeJSON
	d.ensureParentDirs = v.EnsureParentDirs

	return d.validate()

}

func (d *Directory) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(d, unmarshal)
}

// NewDirectory creates a new directory with the given path, mode, user and group.
// user and group can be either a string (user name/group name), an int64 (UID/GID) or nil.
func NewDirectory(path string, mode *os.FileMode, user interface{}, group interface{}, ensureParentDirs bool) (*Directory, error) {
	baseNode, err := newBaseFsNode(path, mode, user, group)

	if err != nil {
		return nil, err
	}

	return &Directory{
		baseFsNode:       *baseNode,
		ensureParentDirs: ensureParentDirs,
	}, nil
}
