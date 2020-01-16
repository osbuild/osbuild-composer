package common

import (
	"encoding/json"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestJSONConversionsComposeRequest(t *testing.T) {
	cases := []struct{
		input string
		expectedConversionResult ComposeRequest
	}{
		// 1st case
		{
			`
			{
				"blueprint_name": "foo",
				"uuid": "789b4d42-da1a-49c9-a20c-054da3bb6c82",
				"distro": "fedora-31",
				"arch": "x86_64",
				"requested_images": [
					{
						"image_type": "AWS",
						"upload_targets": ["AWS EC2"]
					}
				]
			}
			`,
			ComposeRequest{
				BlueprintName: "foo",
				ComposeID:     uuid.UUID{},
				Distro:        Fedora31,
				Arch:          X86_64,
				RequestedImages: []ImageRequest{{
					ImgType:  Aws,
					UpTarget: []UploadTarget{EC2},
				}},
			},
		},
		// 2nd case
		{
			`
			{
				"blueprint_name": "bar",
				"uuid": "789b4d42-da1a-49c9-a20c-054da3bb6c82",
				"distro": "rhel-8.2",
				"arch": "aarch64",
				"requested_images": [
					{
						"image_type": "Azure",
						"upload_targets": ["Azure storage", "AWS EC2"]
					}
				]
			}
			`,
			ComposeRequest{
				BlueprintName: "bar",
				ComposeID:     uuid.UUID{},
				Distro:        RHEL82,
				Arch:          Aarch64,
				RequestedImages: []ImageRequest{{
					ImgType:  Azure,
					UpTarget: []UploadTarget{AzureStorage, EC2},
				}},
			},
		},
	}

	for _, c := range cases {
		// Test unmashaling the JSON from the string above
		var inputStringAsStruct *ComposeRequest
		err := json.Unmarshal([]byte(c.input), &inputStringAsStruct)
		if err != nil {
			t.Fatal("Failed ot unmarshal ComposeRequest:", err)
		}
		if reflect.DeepEqual(inputStringAsStruct, c.expectedConversionResult) {
			t.Error("Unmarshaled compose request is not the one expected")
		}

		// Test marshaling the expected structure into JSON byte array, but since JSON package in golang std lib
		// does not have a canonical form (a 3rd party library is necessary) I convert it back to struct and
		// compare the resulting structure with the input one
		data, err := json.Marshal(c.expectedConversionResult)
		if err != nil {
			t.Fatal("Failed ot marshal ComposeRequest:", err)
		}
		var expectedResultAfterMarshaling *ComposeRequest
		err = json.Unmarshal(data, &expectedResultAfterMarshaling)
		if err != nil {
			t.Fatal("Failed ot unmarshal ComposeRequest:", err, ", input:", string(data))
		}
		if reflect.DeepEqual(expectedResultAfterMarshaling, c.expectedConversionResult) {
			t.Error("Marshaled compose request is not the one expected")
		}
	}

}


