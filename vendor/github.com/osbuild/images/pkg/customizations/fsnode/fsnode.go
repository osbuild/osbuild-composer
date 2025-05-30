package fsnode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/osbuild/images/internal/common"
)

const usernameRegex = `^[A-Za-z0-9_.][A-Za-z0-9_.-]{0,31}$`
const groupnameRegex = `^[A-Za-z0-9_][A-Za-z0-9_-]{0,31}$`

type baseFsNodeJSON struct {
	Path  string
	Mode  *os.FileMode
	User  interface{}
	Group interface{}
}

type baseFsNode struct {
	baseFsNodeJSON
}

func (f *baseFsNode) Path() string {
	if f == nil {
		return ""
	}
	return f.baseFsNodeJSON.Path
}

func (f *baseFsNode) Mode() *os.FileMode {
	if f == nil {
		return nil
	}
	return f.baseFsNodeJSON.Mode
}

// User can return either a string (user name) or an int64 (UID)
func (f *baseFsNode) User() interface{} {
	if f == nil {
		return nil
	}
	return f.baseFsNodeJSON.User
}

// Group can return either a string (group name) or an int64 (GID)
func (f *baseFsNode) Group() interface{} {
	if f == nil {
		return nil
	}
	return f.baseFsNodeJSON.Group
}

func newBaseFsNode(path string, mode *os.FileMode, user interface{}, group interface{}) (*baseFsNode, error) {
	node := &baseFsNode{
		baseFsNodeJSON: baseFsNodeJSON{
			Path:  path,
			Mode:  mode,
			User:  user,
			Group: group,
		},
	}

	err := node.validate()
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (f *baseFsNode) validate() error {
	return f.baseFsNodeJSON.validate()
}

func (f *baseFsNodeJSON) validate() error {
	// Check that the path is valid
	if f.Path == "" {
		return fmt.Errorf("path must not be empty")
	}
	if f.Path[0] != '/' {
		return fmt.Errorf("path must be absolute")
	}
	if f.Path[len(f.Path)-1] == '/' {
		return fmt.Errorf("path must not end with a slash")
	}
	if f.Path != filepath.Clean(f.Path) {
		return fmt.Errorf("path must be canonical")
	}

	// Check that the mode is valid
	if f.Mode != nil && *f.Mode&os.ModeType != 0 {
		return fmt.Errorf("mode must not contain file type bits")
	}

	// Check that the user and group are valid
	switch user := f.User.(type) {
	case string:
		nameRegex := regexp.MustCompile(usernameRegex)
		if !nameRegex.MatchString(user) {
			return fmt.Errorf("user name %q doesn't conform to validating regex (%s)", user, nameRegex.String())
		}
	case float64:
		if user != float64(int64(user)) {
			return fmt.Errorf("user ID must be int")
		}
		if user < 0 {
			return fmt.Errorf("user ID must be non-negative")
		}
	case int64:
		if user < 0 {
			return fmt.Errorf("user ID must be non-negative")
		}
	case nil:
		// user is not set
	default:
		return fmt.Errorf("user must be either a string or an int64, got %T", user)
	}

	switch group := f.Group.(type) {
	case string:
		nameRegex := regexp.MustCompile(groupnameRegex)
		if !nameRegex.MatchString(group) {
			return fmt.Errorf("group name %q doesn't conform to validating regex (%s)", group, nameRegex.String())
		}
	case float64:
		if group != float64(int64(group)) {
			return fmt.Errorf("group ID must be int")
		}
		if group < 0 {
			return fmt.Errorf("group ID must be non-negative")
		}
	case int64:
		if group < 0 {
			return fmt.Errorf("group ID must be non-negative")
		}
	case nil:
		// group is not set
	default:
		return fmt.Errorf("group must be either a string or an int64, got %T", group)
	}

	return nil
}

func (f *baseFsNode) UnmarshalJSON(data []byte) error {
	var fv baseFsNodeJSON
	dec := json.NewDecoder(bytes.NewBuffer(data))
	if err := dec.Decode(&fv); err != nil {
		return err
	}
	if err := fv.validate(); err != nil {
		return err
	}
	f.baseFsNodeJSON = fv

	return nil

}

func (f *baseFsNode) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(f, unmarshal)
}
