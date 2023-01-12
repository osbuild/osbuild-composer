package fsnode

import (
	"fmt"
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestBaseFsNodeValidate(t *testing.T) {
	testCases := []struct {
		Node  baseFsNode
		Error bool
	}{
		// PATH
		// relative path is not allowed
		{
			Node: baseFsNode{
				path: "relative/path/file",
			},
			Error: true,
		},
		// path ending with slash is not allowed
		{
			Node: baseFsNode{
				path: "/dir/with/trailing/slash/",
			},
			Error: true,
		},
		// empty path is not allowed
		{
			Node: baseFsNode{
				path: "",
			},
			Error: true,
		},
		// path must be canonical
		{
			Node: baseFsNode{
				path: "/dir/../file",
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				path: "/dir/./file",
			},
			Error: true,
		},
		// valid paths
		{
			Node: baseFsNode{
				path: "/etc/file",
			},
		},
		{
			Node: baseFsNode{
				path: "/etc/dir",
			},
		},
		// MODE
		// invalid mode
		{
			Node: baseFsNode{
				path: "/etc/file",
				mode: common.ToPtr(os.FileMode(os.ModeDir)),
			},
			Error: true,
		},
		// valid mode
		{
			Node: baseFsNode{
				path: "/etc/file",
				mode: common.ToPtr(os.FileMode(0o644)),
			},
		},
		// USER
		// invalid user
		{
			Node: baseFsNode{
				path: "/etc/file",
				user: "",
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				path: "/etc/file",
				user: "invalid@@@user",
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				path: "/etc/file",
				user: int64(-1),
			},
			Error: true,
		},
		// valid user
		{
			Node: baseFsNode{
				path: "/etc/file",
				user: "osbuild",
			},
		},
		{
			Node: baseFsNode{
				path: "/etc/file",
				user: int64(0),
			},
		},
		// GROUP
		// invalid group
		{
			Node: baseFsNode{
				path:  "/etc/file",
				group: "",
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				path:  "/etc/file",
				group: "invalid@@@group",
			},
			Error: true,
		},
		{
			Node: baseFsNode{
				path:  "/etc/file",
				group: int64(-1),
			},
			Error: true,
		},
		// valid group
		{
			Node: baseFsNode{
				path:  "/etc/file",
				group: "osbuild",
			},
		},
		{
			Node: baseFsNode{
				path:  "/etc/file",
				group: int64(0),
			},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			err := testCase.Node.validate()
			if testCase.Error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
