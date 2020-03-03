package common

import (
	"encoding/json"
	"testing"
)

func TestImageType_UnmarshalJSON(t *testing.T) {
	dict := struct {
		ImageTypes []ImageType `json:"image_types"`
	}{}
	input := `{"image_types":["qcow2", "Alibaba"]}`
	err := json.Unmarshal([]byte(input), &dict)
	if err != nil {
		t.Fatal(err)
	}
	if dict.ImageTypes[0] != Qcow2Generic {
		t.Fatal("failed to umarshal image type qcow2; got tag:", dict.ImageTypes[0])
	}
	if dict.ImageTypes[1] != Alibaba {
		t.Fatal("failed to umarshal image type Alibaba; got tag:", dict.ImageTypes[0])
	}
}
