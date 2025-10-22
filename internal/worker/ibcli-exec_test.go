package worker_test

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/stretchr/testify/assert"
)

func TestRunImageBuilderManifestCall(t *testing.T) {

	type testCase struct {
		args     worker.ImageBuilderArgs
		extraEnv []string

		expCall []string
	}

	testCases := map[string]testCase{
		"empty": {
			expCall: []string{"image-builder", "manifest", "--distro", "", "--arch", "", "--", ""}, // TODO: make this an error
		},

		"simple": {
			args: worker.ImageBuilderArgs{
				Distro:    "centos-9",
				Arch:      "x86_64",
				ImageType: "qcow2",
			},
			expCall: []string{"image-builder", "manifest", "--distro", "centos-9", "--arch", "x86_64", "--", "qcow2"},
		},

		"with-blueprint": {
			args: worker.ImageBuilderArgs{
				Distro:    "centos-10",
				Arch:      "x86_64",
				ImageType: "qcow2",
				Blueprint: &blueprint.Blueprint{
					Customizations: &blueprint.Customizations{
						Hostname: common.ToPtr("testvm"),
						Kernel: &blueprint.KernelCustomization{
							Append: "quiet splash",
						},
					},
				},
			},
			expCall: []string{"image-builder", "manifest", "--distro", "centos-10", "--arch", "x86_64", "--blueprint", "<BLUEPRINTPATH>", "--", "qcow2"},
		},

		"with-env": {
			args: worker.ImageBuilderArgs{
				Distro:    "rhel-10.1",
				Arch:      "aarch64",
				ImageType: "ami",
			},
			extraEnv: []string{"OSBUILD_EXPERIMENTAL_WHATEVER=1"},
			expCall:  []string{"image-builder", "manifest", "--distro", "rhel-10.1", "--arch", "aarch64", "--", "ami"},
		},

		"with-blueprint-and-env": {
			args: worker.ImageBuilderArgs{
				Distro:    "rhel-9.10",
				Arch:      "aarch64",
				ImageType: "azure-rhui",
				Blueprint: &blueprint.Blueprint{
					Customizations: &blueprint.Customizations{
						Hostname: common.ToPtr("image-builder"),
						Timezone: &blueprint.TimezoneCustomization{
							Timezone: common.ToPtr("Europe/Berlin"),
						},
					},
				},
			},
			extraEnv: []string{"OSBUILD_EXPERIMENTAL_WHATEVER=1"},
			expCall:  []string{"image-builder", "manifest", "--distro", "rhel-9.10", "--arch", "aarch64", "--blueprint", "<BLUEPRINTPATH>", "--", "azure-rhui"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			expCall := tc.expCall

			var actualCall []string
			var cmd *exec.Cmd
			var onDiskBP *blueprint.Blueprint
			worker.MockExecCommand(func(name string, arg ...string) *exec.Cmd {
				actualCall = append([]string{name}, arg...)

				// The blueprint path is a random temporary directory, so let's
				// search for it and replace the path in the expected args
				bpPathIdx := slices.Index(actualCall, "--blueprint") + 1
				if bpPathIdx > 0 {
					bpPath := actualCall[bpPathIdx]
					expCall[bpPathIdx] = bpPath

					// load the blueprint and compare it with the original
					bpFile, err := os.Open(bpPath)
					assert.NoError(err)
					defer bpFile.Close()

					bpFileContents, err := io.ReadAll(bpFile)
					assert.NoError(err)
					assert.NoError(json.Unmarshal(bpFileContents, &onDiskBP))
				}

				// return a real exec.Command() result so that the output
				// buffer reading doesn't fail
				cmd = exec.Command("/usr/bin/true")
				return cmd
			})

			_, _ = worker.RunImageBuilderManifest(tc.args, tc.extraEnv, os.Stderr)

			assert.Equal(expCall, actualCall)
			assert.Subset(cmd.Env, tc.extraEnv)
			assert.Equal(tc.args.Blueprint, onDiskBP)
		})
	}
}
