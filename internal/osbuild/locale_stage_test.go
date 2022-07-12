package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLocaleStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.locale",
		Options: &LocaleStageOptions{},
	}
	actualStage := NewLocaleStage(&LocaleStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}
