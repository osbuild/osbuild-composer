package osbuild

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewPwqualityConfStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.pwquality.conf",
		Options: &PwqualityConfStageOptions{},
	}
	actualStage := NewPwqualityConfStage(&PwqualityConfStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestJsonPwqualityConfStage(t *testing.T) {
	// First test that the JSON can be parsed into the expected structure.
	expectedOptions := PwqualityConfStageOptions{
		Config: PwqualityConfConfig{
			Minlen:   common.IntToPtr(9),
			Minclass: common.IntToPtr(0),
			Dcredit:  common.IntToPtr(1),
		},
	}
	inputString := `{
		"config": {
		  "minlen": 9,
		  "minclass": 0,
		  "dcredit": 1
		}
	  }`
	var inputOptions PwqualityConfStageOptions
	err := json.Unmarshal([]byte(inputString), &inputOptions)
	assert.NoError(t, err, "failed to parse JSON yum config")
	assert.True(t, reflect.DeepEqual(expectedOptions, inputOptions))

	// Second try the other way around with stress on missing values
	// for those parameters that the user didn't specify.
	inputOptions = PwqualityConfStageOptions{
		Config: PwqualityConfConfig{
			Minlen:   common.IntToPtr(9),
			Minclass: common.IntToPtr(0),
		},
	}
	expectedString := `{"config":{"minlen":9,"minclass":0}}`
	inputBytes, err := json.Marshal(inputOptions)
	assert.NoError(t, err, "failed to marshal sshd config into JSON")
	assert.Equal(t, expectedString, string(inputBytes))
}
