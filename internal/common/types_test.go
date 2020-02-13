package common

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"reflect"
	"testing"
)

func TestJSONConversionsComposeRequest(t *testing.T) {
	cases := []struct {
		input                    string
		expectedConversionResult ComposeRequest
	}{
		// 1st case
		{
			`
			{
				"blueprint": {
					"name": "",
					"description": "",
					"packages": [],
					"modules": [],
					"groups": []
				},
				"uuid": "789b4d42-da1a-49c9-a20c-054da3bb6c82",
				"distro": "fedora-31",
				"arch": "x86_64",
				"repositories": [],
				"requested_images": [
					{
						"image_type": "AWS",
						"upload_targets": ["AWS EC2"]
					}
				]
			}
			`,
			ComposeRequest{
				Blueprint: blueprint.Blueprint{
					Name:           "",
					Description:    "",
					Version:        "",
					Packages:       nil,
					Modules:        nil,
					Groups:         nil,
					Customizations: nil,
				},
				ComposeID:    uuid.UUID{},
				Distro:       Fedora31,
				Arch:         X86_64,
				Repositories: []rpmmd.RepoConfig{},
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
				"blueprint": {
					"name": "",
					"description": "",
					"packages": [],
					"modules": [],
					"groups": []
				},
				"uuid": "789b4d42-da1a-49c9-a20c-054da3bb6c82",
				"distro": "rhel-8.2",
				"arch": "aarch64",
				"repositories": [],
				"requested_images": [
					{
						"image_type": "Azure",
						"upload_targets": ["Azure storage", "AWS EC2"]
					}
				]
			}
			`,
			ComposeRequest{
				Blueprint: blueprint.Blueprint{
					Name:           "",
					Description:    "",
					Version:        "",
					Packages:       nil,
					Modules:        nil,
					Groups:         nil,
					Customizations: nil,
				},
				ComposeID:    uuid.UUID{},
				Distro:       RHEL82,
				Arch:         Aarch64,
				Repositories: []rpmmd.RepoConfig{},
				RequestedImages: []ImageRequest{{
					ImgType:  Azure,
					UpTarget: []UploadTarget{AzureStorage, EC2},
				}},
			},
		},
	}

	for n, c := range cases {
		// Test unmashaling the JSON from the string above
		var inputStringAsStruct *ComposeRequest
		err := json.Unmarshal([]byte(c.input), &inputStringAsStruct)
		if err != nil {
			t.Fatal("Failed to unmarshal ComposeRequest (", n, "):", err)
		}
		if reflect.DeepEqual(inputStringAsStruct, c.expectedConversionResult) {
			t.Error("Unmarshaled compose request is not the one expected")
		}

		// Test marshaling the expected structure into JSON byte array, but since JSON package in golang std lib
		// does not have a canonical form (a 3rd party library is necessary) I convert it back to struct and
		// compare the resulting structure with the input one
		data, err := json.Marshal(c.expectedConversionResult)
		if err != nil {
			t.Fatal("Failed to marshal ComposeRequest:", err)
		}
		var expectedResultAfterMarshaling *ComposeRequest
		err = json.Unmarshal(data, &expectedResultAfterMarshaling)
		if err != nil {
			t.Fatal("Failed to unmarshal ComposeRequest:", err, ", input:", string(data))
		}
		if reflect.DeepEqual(expectedResultAfterMarshaling, c.expectedConversionResult) {
			t.Error("Marshaled compose request is not the one expected")
		}
	}
}

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
