package common

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJSONConversions(t *testing.T) {
	type TestJson struct {
		Ibs ImageBuildState `json:"ibs"`
		Cs  ComposeState    `json:"cs"`
	}
	typedCases := []TestJson{
		{
			Ibs: IBWaiting,
			Cs:  CWaiting,
		},
		{
			Ibs: IBRunning,
			Cs:  CFailed,
		},
	}
	strCases := []string{
		`{"ibs": "WAITING", "cs": "WAITING"}`,
		`{"ibs": "RUNNING", "cs": "FAILED"}`,
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
