package osbuild

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewYumConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.yum.config",
		Options: &YumConfigStageOptions{},
	}
	actualStage := NewYumConfigStage(&YumConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestJsonYumConfigStage(t *testing.T) {
	expectedOptions := YumConfigStageOptions{
		Config: &YumConfigConfig{
			HttpCaching: common.StringToPtr("packages"),
		},
		Plugins: &YumConfigPlugins{
			&YumConfigPluginsLangpacks{
				Locales: []string{"en_US.UTF-8"},
			},
		},
	}
	inputString := `{"config": {
		"http_caching": "packages"
	  },
	  "plugins": {
		"langpacks": {
		  "locales": [
			"en_US.UTF-8"
		  ]
		}
	  }}`
	var inputOptions YumConfigStageOptions
	err := json.Unmarshal([]byte(inputString), &inputOptions)
	assert.NoError(t, err, "failed to parse JSON yum config")
	assert.True(t, reflect.DeepEqual(expectedOptions, inputOptions))

	inputOptions = YumConfigStageOptions{
		Config: &YumConfigConfig{
			HttpCaching: common.StringToPtr("packages"),
		},
	}
	expectedString := `{"config":{"http_caching":"packages"}}`
	inputBytes, err := json.Marshal(inputOptions)
	assert.NoError(t, err, "failed to marshal YUM config into JSON")
	assert.Equal(t, expectedString, string(inputBytes))
}

func TestYumConfigValidate(t *testing.T) {
	tests := []struct {
		options YumConfigStageOptions
		valid   bool
	}{
		{
			YumConfigStageOptions{},
			true,
		},
		{
			YumConfigStageOptions{
				Plugins: &YumConfigPlugins{
					Langpacks: &YumConfigPluginsLangpacks{
						Locales: []string{},
					},
				},
			},
			false,
		},
		{
			YumConfigStageOptions{
				Plugins: &YumConfigPlugins{
					Langpacks: &YumConfigPluginsLangpacks{
						Locales: []string{"en_US.UTF-8"},
					},
				},
			},
			true,
		},
		{
			YumConfigStageOptions{
				Config: &YumConfigConfig{
					HttpCaching: common.StringToPtr(""),
				},
			},
			false,
		},
		{
			YumConfigStageOptions{
				Config: &YumConfigConfig{
					HttpCaching: common.StringToPtr("all"),
				},
			},
			true,
		},
	}
	for _, test := range tests {
		if test.valid {
			require.NotPanics(t, func() { NewYumConfigStage(&test.options) })
		} else {
			require.Panics(t, func() { NewYumConfigStage(&test.options) })
		}
	}
}
