package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSysctldStageOptions(t *testing.T) {
	filename := "example.conf"
	config := []SysctldConfigLine{{
		Key:   "net.ipv4.conf.default.rp_filter",
		Value: "2",
	}}

	expectedOptions := &SysctldStageOptions{
		Filename: filename,
		Config:   config,
	}
	actualOptions := NewSysctldStageOptions(filename, config)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewSysctldStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.sysctld",
		Options: &SysctldStageOptions{},
	}
	actualStage := NewSysctldStage(&SysctldStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestSysctldStageOptions_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options SysctldStageOptions
	}{
		{
			name:    "empty-options",
			options: SysctldStageOptions{},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}

func TestSysctldConfigLine_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options SysctldConfigLine
	}{
		{
			name: "no-value-without-prefix",
			options: SysctldConfigLine{
				Key: "key-without-prefix",
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
