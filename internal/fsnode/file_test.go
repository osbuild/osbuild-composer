package fsnode

import (
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestFileIsDir(t *testing.T) {
	file, err := NewFile("/etc/file", nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.False(t, file.IsDir())
}

func TestNewFile(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		data     []byte
		mode     *os.FileMode
		user     interface{}
		group    interface{}
		expected *File
	}{
		{
			name:     "empty-file",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{path: "/etc/file", mode: nil, user: nil, group: nil}, data: nil},
		},
		{
			name:     "file-with-data",
			path:     "/etc/file",
			data:     []byte("data"),
			mode:     nil,
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{path: "/etc/file", mode: nil, user: nil, group: nil}, data: []byte("data")},
		},
		{
			name:     "file-with-mode",
			path:     "/etc/file",
			data:     nil,
			mode:     common.ToPtr(os.FileMode(0644)),
			user:     nil,
			group:    nil,
			expected: &File{baseFsNode: baseFsNode{path: "/etc/file", mode: common.ToPtr(os.FileMode(0644)), user: nil, group: nil}, data: nil},
		},
		{
			name:     "file-with-user-and-group-string",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     "user",
			group:    "group",
			expected: &File{baseFsNode: baseFsNode{path: "/etc/file", mode: nil, user: "user", group: "group"}, data: nil},
		},
		{
			name:     "file-with-user-and-group-int64",
			path:     "/etc/file",
			data:     nil,
			mode:     nil,
			user:     int64(1000),
			group:    int64(1000),
			expected: &File{baseFsNode: baseFsNode{path: "/etc/file", mode: nil, user: int64(1000), group: int64(1000)}, data: nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := NewFile(tc.path, tc.mode, tc.user, tc.group, tc.data)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, file)
		})
	}
}
