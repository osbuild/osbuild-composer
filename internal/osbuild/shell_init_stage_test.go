package osbuild

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidShellInitStageOptions(t *testing.T) {
	tests := []ShellInitStageOptions{
		{
			Files: map[string]ShellInitFile{
				"filename": {
					Env: []EnvironmentVariable{
						{
							Key:   "KEY",
							Value: "value",
						},
						{
							Key:   "KEY2",
							Value: "value2",
						},
						{
							Key:   "EMPTY",
							Value: "",
						},
					},
				},
				"filename2": {
					Env: []EnvironmentVariable{
						{
							Key:   "KEY21",
							Value: "value21",
						},
						{
							Key:   "KEY22",
							Value: "value22",
						},
						{
							Key:   "EMPTY",
							Value: "",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"gawk.sh": {
					Env: []EnvironmentVariable{
						{
							Key:   "AWKPATH",
							Value: "$AWKPATH:$*",
						},
						{
							Key:   "AWKLIBPATH",
							Value: "$AWKLIBPATH:$*",
						},
					},
				},
				"flatpak.sh": {
					Env: []EnvironmentVariable{
						{
							Key:   "XDG_DATA_DIRS",
							Value: "${new_dirs:+${new_dirs}:}${XDG_DATA_DIRS:-/usr/local/share:/usr/share}",
						},
					},
				},
			},
		},
	}

	assert := assert.New(t)
	for idx := range tests {
		tt := tests[idx]
		name := fmt.Sprintf("ValidShellInitStage-%d", idx)
		t.Run(name, func(t *testing.T) {
			assert.NoErrorf(tt.validate(), "%q returned an error [idx: %d]", name, idx)
			assert.NotPanics(func() { NewShellInitStage(&tt) })
		})
	}
}

func TestInvalidShellInitStageOptions(t *testing.T) {
	tests := []ShellInitStageOptions{
		{
			Files: map[string]ShellInitFile{

				"path/filename": {
					Env: []EnvironmentVariable{
						{
							Key:   "DOESNT",
							Value: "matter",
						},
					},
				},
				"ok": {
					Env: []EnvironmentVariable{
						{
							Key:   "EMPTYOK",
							Value: "",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"gawk.sh": {
					Env: []EnvironmentVariable{
						{
							Key:   "",
							Value: "badkey",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"$FILENAME": {
					Env: []EnvironmentVariable{
						{
							Key:   "BAD",
							Value: "filename",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"FILENAME": {
					Env: []EnvironmentVariable{
						{
							Key:   "bad.var",
							Value: "okval",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"FILENAME": {
					Env: []EnvironmentVariable{
						{
							Key:   "BAD.VAR",
							Value: "okval",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"me.sh": {
					Env: []EnvironmentVariable{
						{
							Key:   "-SH",
							Value: "",
						},
					},
				},
			},
		},
		{
			Files: map[string]ShellInitFile{
				"empty.sh": {},
			},
		},
	}

	assert := assert.New(t)
	for idx := range tests {
		tt := tests[idx]
		name := fmt.Sprintf("InvalidShellInitStage-%d", idx)
		t.Run(name, func(t *testing.T) {
			assert.Errorf(tt.validate(), "%q didn't return an error [idx: %d]", name, idx)
			assert.Panics(func() { NewShellInitStage(&tt) })
		})
	}
}
