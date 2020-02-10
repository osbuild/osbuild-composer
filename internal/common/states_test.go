package common

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONConversions(t *testing.T) {
	type TestJson struct {
		Ibs ImageBuildState `json:"ibs"`
		Cs ComposeState `json:"cs"`
	}
	typedCases := []TestJson{
		{
			Ibs: IBWaiting,
			Cs: CWaiting,
		},
		{
			Ibs: IBRunning,
			Cs: CFailed,
		},
	}
	strCases := []string{
		`{"ibs": "WAITING", "cs": "WAITING"}`,
		`{"ibs": "RUNNING", "cs": "FAILED"}`,
	}

	for n, c := range strCases {
		var inputStringAsStruct *TestJson
		err := json.Unmarshal([]byte(c), &inputStringAsStruct)
		if err != nil {
			t.Fatal("Failed to unmarshal:", err)
		}
		if reflect.DeepEqual(inputStringAsStruct, typedCases[n]) {
			t.Error("Unmarshaled compose request is not the one expected")
		}
	}

	var byteArrays [][]byte
	for _, c := range typedCases {
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal("Failed to marshal state:", err)
		}
		byteArrays = append(byteArrays, data)
	}
	for n, b := range byteArrays {
		var inputStringAsStruct *TestJson
		err := json.Unmarshal(b, &inputStringAsStruct)
		if err != nil {
			t.Fatal("Failed to unmarshal:", err)
		}
		if reflect.DeepEqual(inputStringAsStruct, typedCases[n]) {
			t.Error("Unmarshaled compose request is not the one expected")
		}
	}
	
}
