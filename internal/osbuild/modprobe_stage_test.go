package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewModprobeStage(t *testing.T) {
	stageOptions := &ModprobeStageOptions{
		Filename: "testing.conf",
		Commands: ModprobeConfigCmdList{
			NewModprobeConfigCmdBlacklist("testing_module"),
		},
	}
	expectedStage := &Stage{
		Type:    "org.osbuild.modprobe",
		Options: stageOptions,
	}
	actualStage := NewModprobeStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestModprobeStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options ModprobeStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: ModprobeStageOptions{},
			err:     true,
		},
		{
			name: "no-commands",
			options: ModprobeStageOptions{
				Filename: "disallow-modules.conf",
				Commands: ModprobeConfigCmdList{},
			},
			err: true,
		},
		{
			name: "no-filename",
			options: ModprobeStageOptions{
				Commands: ModprobeConfigCmdList{NewModprobeConfigCmdBlacklist("module_name")},
			},
			err: true,
		},
		{
			name: "incorrect-filename",
			options: ModprobeStageOptions{
				Filename: "disallow-modules.ccoonnff",
				Commands: ModprobeConfigCmdList{NewModprobeConfigCmdBlacklist("module_name")},
			},
			err: true,
		},
		{
			name: "good-options",
			options: ModprobeStageOptions{
				Filename: "disallow-modules.conf",
				Commands: ModprobeConfigCmdList{NewModprobeConfigCmdBlacklist("module_name")},
			},
			err: false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewModprobeStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewModprobeStage(&tt.options) })
			}
		})
	}
}

func TestNewModprobeConfigCmdBlacklist(t *testing.T) {
	tests := []struct {
		name       string
		modulename string
		err        bool
	}{
		{
			name:       "empty-modulename",
			modulename: "",
			err:        true,
		},
		{
			name:       "non-empty-modulename",
			modulename: "module_name",
			err:        false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, ModprobeConfigCmdBlacklist{Command: "blacklist", Modulename: tt.modulename}.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewModprobeConfigCmdBlacklist(tt.modulename) })
			} else {
				assert.NoErrorf(t, ModprobeConfigCmdBlacklist{Command: "blacklist", Modulename: tt.modulename}.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewModprobeConfigCmdBlacklist(tt.modulename) })
			}
		})
	}
}

func TestNewModprobeConfigCmdInstall(t *testing.T) {
	tests := []struct {
		name       string
		modulename string
		cmdline    string
		err        bool
	}{
		{
			name:       "empty-modulename",
			modulename: "",
			cmdline:    "/usr/bin/true",
			err:        true,
		},
		{
			name:       "empty-cmdline",
			modulename: "module_name",
			cmdline:    "",
			err:        true,
		},
		{
			name:       "non-empty-modulename",
			modulename: "module_name",
			cmdline:    "/usr/bin/true",
			err:        false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, ModprobeConfigCmdInstall{Command: "install", Modulename: tt.modulename, Cmdline: tt.cmdline}.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewModprobeConfigCmdInstall(tt.modulename, tt.cmdline) })
			} else {
				assert.NoErrorf(t, ModprobeConfigCmdInstall{Command: "install", Modulename: tt.modulename, Cmdline: tt.cmdline}.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewModprobeConfigCmdInstall(tt.modulename, tt.cmdline) })
			}
		})
	}
}
