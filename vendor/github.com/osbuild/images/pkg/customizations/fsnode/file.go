package fsnode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/osbuild/images/internal/common"
)

type File struct {
	baseFsNode
	data []byte

	uri string
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

func (f *File) URI() string {
	return f.uri
}

// NewFile creates a new file with the given path, data, mode, user and group.
// user and group can be either a string (user name/group name), an int64 (UID/GID) or nil.
func NewFile(path string, mode *os.FileMode, user interface{}, group interface{}, data []byte) (*File, error) {
	return newFile(path, mode, user, group, data, "")
}

// NewFleForURI creates a new file from the given "URI" (currently
// only local file are supported).
func NewFileForURI(targetPath string, mode *os.FileMode, user interface{}, group interface{}, uriStr string) (*File, error) {
	uri, err := url.Parse(uriStr)
	if err != nil {
		return nil, err
	}
	switch uri.Scheme {
	case "", "file":
		return newFileForURILocalFile(targetPath, mode, user, group, uri)
	default:
		return nil, fmt.Errorf("unsupported scheme for %v (try file://)", uri)
	}
}

func newFileForURILocalFile(targetPath string, mode *os.FileMode, user interface{}, group interface{}, uri *url.URL) (*File, error) {
	st, err := os.Stat(uri.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot include blueprint file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", uri.Path)
	}
	if mode == nil {
		mode = common.ToPtr(st.Mode())
	}
	// note that user/group are not take from the local file, just
	// default to unset which means root

	return newFile(targetPath, mode, user, group, nil, uri.Path)
}

func newFile(path string, mode *os.FileMode, user interface{}, group interface{}, data []byte, uri string) (*File, error) {
	baseNode, err := newBaseFsNode(path, mode, user, group)

	if err != nil {
		return nil, err
	}

	return &File{
		baseFsNode: *baseNode,
		data:       data,
		uri:        uri,
	}, nil
}
