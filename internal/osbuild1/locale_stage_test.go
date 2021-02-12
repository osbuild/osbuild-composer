package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLocaleStage(t *testing.T) {
	expectedStage := &Stage{
		Name:    "org.osbuild.locale",
		Options: &LocaleStageOptions{},
	}
	actualStage := NewLocaleStage(&LocaleStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}
