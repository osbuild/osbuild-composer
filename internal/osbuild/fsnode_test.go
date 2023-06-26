package osbuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/fsnode"
	"github.com/stretchr/testify/assert"
)

func TestGenFileNodesStages(t *testing.T) {
	fileData1 := []byte("hello world")
	fileData2 := []byte("hello world 2")

	ensureFileCreation := func(file *fsnode.File, err error) *fsnode.File {
		t.Helper()
		assert.NoError(t, err)
		assert.NotNil(t, file)
		return file
	}

	testCases := []struct {
		name     string
		files    []*fsnode.File
		expected []*Stage
	}{
		{
			name:     "empty-files-list",
			files:    []*fsnode.File{},
			expected: nil,
		},
		{
			name:     "nil-files-list",
			files:    nil,
			expected: nil,
		},
		{
			name: "single-file-simple",
			files: []*fsnode.File{
				ensureFileCreation(fsnode.NewFile("/etc/file", nil, nil, nil, []byte(fileData1))),
			},
			expected: []*Stage{
				NewCopyStageSimple(&CopyStageOptions{
					Paths: []CopyStagePath{
						{
							From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", sha256.Sum256(fileData1)),
							To:                "tree:///etc/file",
							RemoveDestination: true,
						},
					},
				}, &CopyStageFilesInputs{
					fmt.Sprintf("file-%x", sha256.Sum256(fileData1)): NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
						NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%x", sha256.Sum256(fileData1)), nil),
					})),
				}),
			},
		},
		{
			name: "multiple-files-simple",
			files: []*fsnode.File{
				ensureFileCreation(fsnode.NewFile("/etc/file", nil, nil, nil, []byte(fileData1))),
				ensureFileCreation(fsnode.NewFile("/etc/file2", nil, nil, nil, []byte(fileData2))),
			},
			expected: []*Stage{
				NewCopyStageSimple(&CopyStageOptions{
					Paths: []CopyStagePath{
						{
							From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", sha256.Sum256(fileData1)),
							To:                "tree:///etc/file",
							RemoveDestination: true,
						},
						{
							From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", sha256.Sum256(fileData2)),
							To:                "tree:///etc/file2",
							RemoveDestination: true,
						},
					},
				}, &CopyStageFilesInputs{
					fmt.Sprintf("file-%x", sha256.Sum256(fileData1)): NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
						NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%x", sha256.Sum256(fileData1)), nil),
					})),
					fmt.Sprintf("file-%x", sha256.Sum256(fileData2)): NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
						NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%x", sha256.Sum256(fileData2)), nil),
					})),
				}),
			},
		},
		{
			name: "multiple-files-with-all-options",
			files: []*fsnode.File{
				ensureFileCreation(fsnode.NewFile("/etc/file", common.ToPtr(os.FileMode(0644)), "root", int64(12345), []byte(fileData1))),
				ensureFileCreation(fsnode.NewFile("/etc/file2", common.ToPtr(os.FileMode(0755)), int64(12345), "root", []byte(fileData2))),
			},
			expected: []*Stage{
				NewCopyStageSimple(&CopyStageOptions{
					Paths: []CopyStagePath{
						{
							From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", sha256.Sum256(fileData1)),
							To:                "tree:///etc/file",
							RemoveDestination: true,
						},
						{
							From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", sha256.Sum256(fileData2)),
							To:                "tree:///etc/file2",
							RemoveDestination: true,
						},
					},
				}, &CopyStageFilesInputs{
					fmt.Sprintf("file-%x", sha256.Sum256(fileData1)): NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
						NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%x", sha256.Sum256(fileData1)), nil),
					})),
					fmt.Sprintf("file-%x", sha256.Sum256(fileData2)): NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
						NewFilesInputSourceArrayRefEntry(fmt.Sprintf("sha256:%x", sha256.Sum256(fileData2)), nil),
					})),
				}),
				NewChmodStage(&ChmodStageOptions{
					Items: map[string]ChmodStagePathOptions{
						"/etc/file": {
							Mode: fmt.Sprintf("%#o", os.FileMode(0644)),
						},
						"/etc/file2": {
							Mode: fmt.Sprintf("%#o", os.FileMode(0755)),
						},
					},
				}),
				NewChownStage(&ChownStageOptions{
					Items: map[string]ChownStagePathOptions{
						"/etc/file": {
							User:  "root",
							Group: int64(12345),
						},
						"/etc/file2": {
							User:  int64(12345),
							Group: "root",
						},
					},
				}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotStages := GenFileNodesStages(tc.files)
			assert.EqualValues(t, tc.expected, gotStages)
		})
	}
}

func TestGenDirectoryNodesStages(t *testing.T) {

	ensureDirCreation := func(dir *fsnode.Directory, err error) *fsnode.Directory {
		t.Helper()
		assert.NoError(t, err)
		assert.NotNil(t, dir)
		return dir
	}

	testCases := []struct {
		name     string
		dirs     []*fsnode.Directory
		expected []*Stage
	}{
		{
			name:     "empty-dirs-list",
			dirs:     []*fsnode.Directory{},
			expected: nil,
		},
		{
			name:     "nil-dirs-list",
			dirs:     nil,
			expected: nil,
		},
		{
			name: "single-dir-simple",
			dirs: []*fsnode.Directory{
				ensureDirCreation(fsnode.NewDirectory("/etc/dir", nil, nil, nil, false)),
			},
			expected: []*Stage{
				NewMkdirStage(&MkdirStageOptions{
					Paths: []MkdirStagePath{
						{
							Path:    "/etc/dir",
							ExistOk: true,
						},
					},
				}),
			},
		},
		{
			name: "multiple-dirs-simple",
			dirs: []*fsnode.Directory{
				ensureDirCreation(fsnode.NewDirectory("/etc/dir", nil, nil, nil, false)),
				ensureDirCreation(fsnode.NewDirectory("/etc/dir2", nil, nil, nil, false)),
			},
			expected: []*Stage{
				NewMkdirStage(&MkdirStageOptions{
					Paths: []MkdirStagePath{
						{
							Path:    "/etc/dir",
							ExistOk: true,
						},
						{
							Path:    "/etc/dir2",
							ExistOk: true,
						},
					},
				}),
			},
		},
		{
			name: "multiple-dirs-with-all-options",
			dirs: []*fsnode.Directory{
				ensureDirCreation(fsnode.NewDirectory("/etc/dir", common.ToPtr(os.FileMode(0700)), "root", int64(12345), true)),
				ensureDirCreation(fsnode.NewDirectory("/etc/dir2", common.ToPtr(os.FileMode(0755)), int64(12345), "root", false)),
				ensureDirCreation(fsnode.NewDirectory("/etc/dir3", nil, nil, nil, true)),
			},
			expected: []*Stage{
				NewMkdirStage(&MkdirStageOptions{
					Paths: []MkdirStagePath{
						{
							Path:    "/etc/dir",
							Parents: true,
							ExistOk: false,
						},
						{
							Path:    "/etc/dir2",
							ExistOk: false,
						},
						{
							Path:    "/etc/dir3",
							Parents: true,
							ExistOk: true,
						},
					},
				}),
				NewChmodStage(&ChmodStageOptions{
					Items: map[string]ChmodStagePathOptions{
						"/etc/dir": {
							Mode: fmt.Sprintf("%#o", os.FileMode(0700)),
						},
						"/etc/dir2": {
							Mode: fmt.Sprintf("%#o", os.FileMode(0755)),
						},
					},
				}),
				NewChownStage(&ChownStageOptions{
					Items: map[string]ChownStagePathOptions{
						"/etc/dir": {
							User:  "root",
							Group: int64(12345),
						},
						"/etc/dir2": {
							User:  int64(12345),
							Group: "root",
						},
					},
				}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotStages := GenDirectoryNodesStages(tc.dirs)
			assert.EqualValues(t, tc.expected, gotStages)
		})
	}
}
