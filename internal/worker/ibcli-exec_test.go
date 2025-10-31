package worker_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/rpmmd"
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
			expCall: []string{"image-builder", "manifest", "--distro", "centos-10", "--arch", "x86_64", "--blueprint", "<TMPDIR>/blueprint.json", "--", "qcow2"},
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
			expCall:  []string{"image-builder", "manifest", "--distro", "rhel-9.10", "--arch", "aarch64", "--blueprint", "<TMPDIR>/blueprint.json", "--", "azure-rhui"},
		},

		"with-repos": {
			args: worker.ImageBuilderArgs{
				Distro:    "rhel-10.10",
				Arch:      "aarch64",
				ImageType: "azure-rhui",
				Repositories: []rpmmd.RepoConfig{
					{
						Id:       "baseos",
						Name:     "baseos",
						BaseURLs: []string{"https://example.org/baseos"},
					},
				},
			},
			expCall: []string{
				"image-builder",
				"manifest",
				"--distro", "rhel-10.10",
				"--arch", "aarch64",
				"--data-dir", "<TMPDIR>",
				"--",
				"azure-rhui",
			},
		},
		"with-blueprint-and-repos": {
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
				Repositories: []rpmmd.RepoConfig{
					{
						Id:       "baseos",
						Name:     "baseos",
						BaseURLs: []string{"https://example.org/baseos"},
					},
				},
			},
			expCall: []string{
				"image-builder",
				"manifest",
				"--distro", "rhel-9.10",
				"--arch", "aarch64",
				"--blueprint", "<TMPDIR>/blueprint.json",
				"--data-dir", "<TMPDIR>",
				"--",
				"azure-rhui",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			expCall := tc.expCall

			var actualCall []string
			var cmd *exec.Cmd
			worker.MockExecCommand(func(name string, arg ...string) *exec.Cmd {
				actualCall = append([]string{name}, arg...)

				// The blueprint path is under a random temporary directory, so
				// let's search for it and replace the path in the expected
				// args. Also, load the blueprint contents to compare them with
				// the original from the test case.
				var onDiskBP *blueprint.Blueprint
				bpPathIdx := slices.Index(actualCall, "--blueprint") + 1
				if bpPathIdx > 0 {
					bpPath := actualCall[bpPathIdx]
					expCall[bpPathIdx] = bpPath

					bpFile, err := os.Open(bpPath)
					assert.NoError(err)
					defer bpFile.Close()

					bpFileContents, err := io.ReadAll(bpFile)
					assert.NoError(err)
					assert.NoError(json.Unmarshal(bpFileContents, &onDiskBP))
				}
				assert.Equal(tc.args.Blueprint, onDiskBP)

				// The repos path is under a random temporary directory (the
				// datadir), so let's search for it and replace the path in the
				// expected args. Also, load the repos file contents to compare
				// them with the original from the test case.
				var onDiskRepos []rpmmd.RepoConfig
				dataDirIdx := slices.Index(actualCall, "--data-dir") + 1
				if dataDirIdx > 0 {
					dataDir := actualCall[dataDirIdx]
					expCall[dataDirIdx] = dataDir

					reposPath := filepath.Join(dataDir, "repositories", fmt.Sprintf("%s.json", tc.args.Distro))
					reposFile, err := os.Open(reposPath)
					assert.NoError(err)
					defer reposFile.Close()

					reposFileContents, err := io.ReadAll(reposFile)
					assert.NoError(err)
					assert.NoError(json.Unmarshal(reposFileContents, &onDiskRepos))
				}
				assert.Equal(tc.args.Repositories, onDiskRepos)

				// return a real exec.Command() result so that the output
				// buffer reading doesn't fail
				cmd = exec.Command("/usr/bin/true")
				return cmd
			})

			_, err := worker.RunImageBuilderManifest(tc.args, tc.extraEnv, os.Stderr)
			assert.NoError(err)

			assert.Equal(expCall, actualCall)
			assert.Subset(cmd.Env, tc.extraEnv)
		})
	}
}
