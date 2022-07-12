package osbuild

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRhsmStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rhsm",
		Options: &RHSMStageOptions{},
	}
	actualStage := NewRHSMStage(&RHSMStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestRhsmStageJson(t *testing.T) {
	tests := []struct {
		Options    RHSMStageOptions
		JsonString string
	}{
		{
			Options: RHSMStageOptions{
				YumPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			},
			JsonString: `{"yum-plugins":{"product-id":{"enabled":true},"subscription-manager":{"enabled":false}}}`,
		},
		{
			Options: RHSMStageOptions{
				DnfPlugins: &RHSMStageOptionsDnfPlugins{
					ProductID: &RHSMStageOptionsDnfPlugin{
						Enabled: true,
					},
					SubscriptionManager: &RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			},
			JsonString: `{"dnf-plugins":{"product-id":{"enabled":true},"subscription-manager":{"enabled":false}}}`,
		},
		{
			Options: RHSMStageOptions{
				SubMan: &RHSMStageOptionsSubMan{
					Rhsm:      &SubManConfigRHSMSection{},
					Rhsmcertd: &SubManConfigRHSMCERTDSection{},
				},
			},
			JsonString: `{"subscription-manager":{"rhsm":{},"rhsmcertd":{}}}`,
		},
	}
	for _, test := range tests {
		marshaledJson, err := json.Marshal(test.Options)
		require.NoError(t, err, "failed to marshal JSON")
		require.Equal(t, string(marshaledJson), test.JsonString)

		var jsonOptions RHSMStageOptions
		err = json.Unmarshal([]byte(test.JsonString), &jsonOptions)
		require.NoError(t, err, "failed to parse JSON")
		require.True(t, reflect.DeepEqual(test.Options, jsonOptions))
	}
}
