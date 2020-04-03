package common

import (
	"encoding/json"
	"testing"
)

func TestImageType_UnmarshalJSON(t *testing.T) {
	dict := struct {
		ImageTypes []ImageType `json:"image_types"`
	}{}
	input := `{"image_types":["qcow2", "Azure"]}`
	err := json.Unmarshal([]byte(input), &dict)
	if err != nil {
		t.Fatal(err)
	}
	if dict.ImageTypes[0] != Qcow2Generic {
		t.Fatal("failed to umarshal image type qcow2; got tag:", dict.ImageTypes[0])
	}
	if dict.ImageTypes[1] != Azure {
		t.Fatal("failed to umarshal image type Azure; got tag:", dict.ImageTypes[0])
	}
}
