package osbuild1

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAssembler_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name          string
		assembler     Assembler
		data          []byte
		errorExpected bool
	}{
		{
			// invalid JSON - note the missing brace at the end of the string
			name:          "invalid json",
			data:          []byte(`{"name":"org.osbuild.tar","options":{"filename":""}`),
			errorExpected: true,
		},
		{
			// valid JSON, but with an unknown assembler (org.osbuild.foo)
			name:          "unknown assembler",
			data:          []byte(`{"name":"org.osbuild.foo","options":{"bar":null}}`),
			errorExpected: true,
		},
		{
			name:          "missing options",
			data:          []byte(`{"name":"org.osbuild.rawfs"`),
			errorExpected: true,
		},
		{
			name:          "missing name",
			data:          []byte(`{"options":{"bar":null}}`),
			errorExpected: true,
		},
		{
			name: "qemu assembler empty",
			assembler: Assembler{
				Name:    "org.osbuild.qemu",
				Options: &QEMUAssemblerOptions{},
			},
			data: []byte(`{"name":"org.osbuild.qemu","options":{"format":"","filename":"","size":0,"ptuuid":"","pttype":"","partitions":null}}`),
		},
		{
			name: "qemu assembler full",
			assembler: Assembler{
				Name: "org.osbuild.qemu",
				Options: &QEMUAssemblerOptions{
					Format:   "qcow2",
					Filename: "disk.qcow2",
					Size:     2147483648,
					PTUUID:   "0x14fc63d2",
					PTType:   "mbr",
					Partitions: []QEMUPartition{QEMUPartition{
						Start:    2048,
						Bootable: true,
						Filesystem: &QEMUFilesystem{
							Type:       "ext4",
							UUID:       "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
							Label:      "root",
							Mountpoint: "/",
						},
					}},
				},
			},
			data: []byte(`{"name":"org.osbuild.qemu","options":{"format":"qcow2","filename":"disk.qcow2","size":2147483648,"ptuuid":"0x14fc63d2","pttype":"mbr","partitions":[{"start":2048,"bootable":true,"filesystem":{"type":"ext4","uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","label":"root","mountpoint":"/"}}]}}`),
		},
		{
			name: "tar assembler empty",
			assembler: Assembler{
				Name:    "org.osbuild.tar",
				Options: &TarAssemblerOptions{},
			},
			data: []byte(`{"name":"org.osbuild.tar","options":{"filename":""}}`),
		},
		{
			name: "tar assembler full",
			assembler: Assembler{
				Name: "org.osbuild.tar",
				Options: &TarAssemblerOptions{
					Filename:    "root.tar.xz",
					Compression: "xz",
				},
			},
			data: []byte(`{"name":"org.osbuild.tar","options":{"filename":"root.tar.xz","compression":"xz"}}`),
		},
		{
			name: "rawfs assembler empty",
			assembler: Assembler{
				Name:    "org.osbuild.rawfs",
				Options: &RawFSAssemblerOptions{},
			},
			data: []byte(`{"name":"org.osbuild.rawfs","options":{"filename":"","root_fs_uuid":"00000000-0000-0000-0000-000000000000","size":0}}`),
		},
		{
			name: "rawfs assembler full",
			assembler: Assembler{
				Name: "org.osbuild.rawfs",
				Options: &RawFSAssemblerOptions{
					Filename:           "filesystem.img",
					RootFilesystemUUID: uuid.MustParse("76a22bf4-f153-4541-b6c7-0332c0dfaeac"),
					Size:               2147483648,
				},
			},
			data: []byte(`{"name":"org.osbuild.rawfs","options":{"filename":"filesystem.img","root_fs_uuid":"76a22bf4-f153-4541-b6c7-0332c0dfaeac","size":2147483648}}`),
		},
		{
			name: "ostree commit assembler",
			assembler: Assembler{
				Name: "org.osbuild.ostree.commit",
				Options: &OSTreeCommitAssemblerOptions{
					Ref: "foo",
					Tar: OSTreeCommitAssemblerTarOptions{
						Filename: "foo.tar",
					},
				},
			},
			data: []byte(`{"name":"org.osbuild.ostree.commit","options":{"ref":"foo","tar":{"filename":"foo.tar"}}}`),
		},
	}

	assert := assert.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assembler := &tt.assembler
			var gotAssembler Assembler
			err := gotAssembler.UnmarshalJSON(tt.data)
			if tt.errorExpected {
				assert.NotNil(err)
				return
			} else {
				assert.Nil(err)
			}
			gotBytes, err := json.Marshal(assembler)
			assert.Nilf(err, "Could not marshal assembler: %v", err)
			assert.Equal(tt.data, gotBytes)
			assert.Equal(&gotAssembler, assembler)
		})
	}
}

func TestNewQEMUAssembler(t *testing.T) {
	options := &QEMUAssemblerOptions{}
	expectedAssembler := &Assembler{
		Name:    "org.osbuild.qemu",
		Options: &QEMUAssemblerOptions{},
	}
	assert.Equal(t, expectedAssembler, NewQEMUAssembler(options))
}

func TestNewTarAssembler(t *testing.T) {
	options := &TarAssemblerOptions{}
	expectedAssembler := &Assembler{
		Name:    "org.osbuild.tar",
		Options: &TarAssemblerOptions{},
	}
	assert.Equal(t, expectedAssembler, NewTarAssembler(options))
}

func TestNewRawFSAssembler(t *testing.T) {
	options := &RawFSAssemblerOptions{}
	expectedAssembler := &Assembler{
		Name:    "org.osbuild.rawfs",
		Options: &RawFSAssemblerOptions{},
	}
	assert.Equal(t, expectedAssembler, NewRawFSAssembler(options))
}
