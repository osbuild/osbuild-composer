package osbuild2

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewSshdConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.sshd.config",
		Options: &SshdConfigStageOptions{},
	}
	actualStage := NewSshdConfigStage(&SshdConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestJsonSshdConfigStage(t *testing.T) {
	// First test that the JSON can be parsed into the expected structure.
	expectedOptions := SshdConfigStageOptions{
		Config: SshdConfigConfig{
			PasswordAuthentication:          common.BoolToPtr(false),
			ChallengeResponseAuthentication: common.BoolToPtr(false),
			ClientAliveInterval:             common.IntToPtr(180),
		},
	}
	inputString := `{
		"config": {
		  "PasswordAuthentication": false,
		  "ChallengeResponseAuthentication": false,
		  "ClientAliveInterval": 180
		}
	  }`
	var inputOptions SshdConfigStageOptions
	err := json.Unmarshal([]byte(inputString), &inputOptions)
	assert.NoError(t, err, "failed to parse JSON into sshd config")
	assert.True(t, reflect.DeepEqual(expectedOptions, inputOptions))

	// Second try the other way around with stress on missing values
	// for those parameters that the user didn't specify.
	inputOptions = SshdConfigStageOptions{
		Config: SshdConfigConfig{
			PasswordAuthentication: common.BoolToPtr(true),
		},
	}
	expectedString := `{"config":{"PasswordAuthentication":true}}`
	inputBytes, err := json.Marshal(inputOptions)
	assert.NoError(t, err, "failed to marshal sshd config into JSON")
	assert.Equal(t, expectedString, string(inputBytes))
}
