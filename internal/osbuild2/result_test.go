package osbuild2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStageResult_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input   string
		success bool
	}{
		{input: `{}`, success: true},
		{input: `{"success": true}`, success: true},
		{input: `{"success": false}`, success: false},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			var result StageResult
			err := json.Unmarshal([]byte(c.input), &result)
			assert.NoError(t, err)
			assert.Equal(t, c.success, result.Success)
		})
	}
}
