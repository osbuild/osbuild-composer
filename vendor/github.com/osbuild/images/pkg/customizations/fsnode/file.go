package fsnode

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/osbuild/images/internal/common"
)

type File struct {
	baseFsNode
	data []byte
}

func (f *File) Data() []byte {
	if f == nil {
		return nil
	}
	return f.data
}

func (f *File) UnmarshalJSON(data []byte) error {
	var v struct {
		baseFsNodeJSON
		Data string `json:"data,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewBuffer(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	f.baseFsNode.baseFsNodeJSON = v.baseFsNodeJSON
	f.data = []byte(v.Data)

	return f.validate()

}

func (f *File) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(f, unmarshal)
}

// NewFile creates a new file with the given path, data, mode, user and group.
// user and group can be either a string (user name/group name), an int64 (UID/GID) or nil.
func NewFile(path string, mode *os.FileMode, user interface{}, group interface{}, data []byte) (*File, error) {
	baseNode, err := newBaseFsNode(path, mode, user, group)

	if err != nil {
		return nil, err
	}

	return &File{
		baseFsNode: *baseNode,
		data:       data,
	}, nil
}
