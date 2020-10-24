package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONConversions(t *testing.T) {
	type TestJson struct {
		Ibs ImageBuildState `json:"ibs"`
	}
	typedCases := []TestJson{
		{
			Ibs: IBWaiting,
		},
		{
			Ibs: IBRunning,
		},
	}
	strCases := []string{
		`{"ibs": "WAITING"}`,
		`{"ibs": "RUNNING"}`,
	}

	for n, c := range strCases {
		var inputStringAsStruct *TestJson
		err := json.Unmarshal([]byte(c), &inputStringAsStruct)
		assert.NoErrorf(t, err, "Failed to unmarshal: %#v", err)
		assert.Equal(t, inputStringAsStruct, &typedCases[n])
	}

	var byteArrays [][]byte
	for _, c := range typedCases {
		data, err := json.Marshal(c)
		assert.NoError(t, err)
		byteArrays = append(byteArrays, data)
	}
	for n, b := range byteArrays {
		var inputStringAsStruct *TestJson
		err := json.Unmarshal(b, &inputStringAsStruct)
		assert.NoError(t, err)
		assert.Equal(t, inputStringAsStruct, &typedCases[n])
	}

}
