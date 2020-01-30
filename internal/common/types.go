package common

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
)

// CustomJsonConversionError is thrown when parsing strings into enumerations
type CustomJsonConversionError struct {
	reason string
}

func (err *CustomJsonConversionError) Error() string {
	return err.reason
}

func unmarshalHelper(data []byte, jsonError, typeConversionError string, mapping map[string]int) (int, error) {
	var stringInput string
	err := json.Unmarshal(data, &stringInput)
	if err != nil {
		return 0, &CustomJsonConversionError{string(data) + jsonError}
	}
	value, exists := mapping[stringInput]
	if !exists {
		return 0, &CustomJsonConversionError{stringInput + typeConversionError}
	}
	return value, nil
}

func marshalHelper(input int, mapping map[string]int, errorMessage string) ([]byte, error) {
	for k, v := range mapping {
		if v == input {
			return json.Marshal(k)
		}
	}
	return nil, &CustomJsonConversionError{fmt.Sprintf("%d %s", input, errorMessage)}
}

func listHelper(mapping map[string]int) []string {
	ret := make([]string, 0)
	for k,_ := range mapping {
		ret = append(ret, k)
	}
	return ret
}

func existsHelper(mapping map[string]int, testedValue string) bool {
	for k, _ := range mapping {
		if k == testedValue {
			return true
		}
	}
	return false
}

// Architecture represents one of the supported CPU architectures available for images
// produced by osbuild-composer. It is represented as an integer because if it
// was a string it would unmarshal from JSON just fine even in case that the architecture
// was unknown.
type Architecture int

// A list of supported architectures. As the comment above suggests the type system does
// not allow to create a type with a custom set of values, so it is possible to use e.g.
// 56 instead of an architecture, but as opposed to a string it should be obvious that
// hardcoding a number instead of an architecture is just wrong.
//
// NOTE: If you want to add more constants here, don't forget to add a mapping below
const (
	X86_64 Architecture = iota
	Aarch64
	Armv7hl
	I686
	Ppc64le
	S390x
)

// getArchMapping is a helper function that defines the conversion from JSON string value
// to Architecture.
func getArchMapping() map[string]int {
	mapping := map[string]int{
		"x86_64":  int(X86_64),
		"aarch64": int(Aarch64),
		"armv7hl": int(Armv7hl),
		"i686":    int(I686),
		"ppc64le": int(Ppc64le),
		"s390x":   int(S390x),
	}
	return mapping
}

func ListArchitectures() []string {
	return listHelper(getArchMapping())
}

func ArchitectureExists(testedArch string) bool {
	return existsHelper(getArchMapping(), testedArch)
}

// UnmarshalJSON is a custom unmarshaling function to limit the set of allowed values
// in case the input is JSON.
func (arch Architecture) UnmarshalJSON(data []byte) error {
	value, err := unmarshalHelper(data, " is not a valid JSON value", " is not a valid architecture", getArchMapping())
	if err != nil {
		return err
	}
	arch = Architecture(value)
	return nil
}

// MarshalJSON is a custom marshaling function for our custom Architecture type
func (arch Architecture) MarshalJSON() ([]byte, error) {
	return marshalHelper(int(arch), getArchMapping(), "is not a valid architecture tag")
}

type ImageType int

// NOTE: If you want to add more constants here, don't forget to add a mapping below
const (
	Alibaba ImageType = iota
	Azure
	Aws
	GoogleCloud
	HyperV
	LiveISO
	OpenStack
	Qcow2Generic
	Vmware
)

// getArchMapping is a helper function that defines the conversion from JSON string value
// to ImageType.
func getImageTypeMapping() map[string]int {
	mapping := map[string]int{
		"Alibaba":      int(Alibaba),
		"Azure":        int(Azure),
		"AWS":          int(Aws),
		"Google Cloud": int(GoogleCloud),
		"Hyper-V":      int(HyperV),
		"LiveISO":      int(LiveISO),
		"OpenStack":    int(OpenStack),
		"qcow2":        int(Qcow2Generic),
		"VMWare":       int(Vmware),
	}
	return mapping
}

func (imgType ImageType) UnmarshalJSON(data []byte) error {
	value, err := unmarshalHelper(data, " is not a valid JSON value", " is not a valid image type", getImageTypeMapping())
	if err != nil {
		return err
	}
	imgType = ImageType(value)
	return nil
}

func (imgType ImageType) MarshalJSON() ([]byte, error) {
	return marshalHelper(int(imgType), getImageTypeMapping(), "is not a valid image type tag")
}

type Distribution int

// NOTE: If you want to add more constants here, don't forget to add a mapping below
const (
	Fedora30 Distribution = iota
	Fedora31
	RHEL82
)

// getArchMapping is a helper function that defines the conversion from JSON string value
// to Distribution.
func getDistributionMapping() map[string]int {
	mapping := map[string]int{
		"fedora-30": int(Fedora30),
		"fedora-31": int(Fedora31),
		"rhel-8.2":  int(RHEL82),
	}
	return mapping
}


func ListDistributions() []string {
	return listHelper(getDistributionMapping())
}

func DistributionExists(testedDistro string) bool {
	return existsHelper(getDistributionMapping(), testedDistro)
}

func (distro Distribution) UnmarshalJSON(data []byte) error {
	value, err := unmarshalHelper(data, " is not a valid JSON value", " is not a valid distribution", getDistributionMapping())
	if err != nil {
		return err
	}
	distro = Distribution(value)
	return nil
}

func (distro Distribution) MarshalJSON() ([]byte, error) {
	return marshalHelper(int(distro), getDistributionMapping(), "is not a valid distribution tag")
}

type UploadTarget int

// NOTE: If you want to add more constants here, don't forget to add a mapping below
const (
	EC2          UploadTarget = iota
	AzureStorage              // I mention "storage" explicitly because we might want to support gallery as well
)

// getArchMapping is a helper function that defines the conversion from JSON string value
// to UploadTarget.
func getUploadTargetMapping() map[string]int {
	mapping := map[string]int{
		"AWS EC2":       int(EC2),
		"Azure storage": int(AzureStorage),
	}
	return mapping
}

func (ut UploadTarget) UnmarshalJSON(data []byte) error {
	value, err := unmarshalHelper(data, " is not a valid JSON value", " is not a valid upload target", getUploadTargetMapping())
	if err != nil {
		return err
	}
	ut = UploadTarget(value)
	return nil
}

func (ut UploadTarget) MarshalJSON() ([]byte, error) {
	return marshalHelper(int(ut), getUploadTargetMapping(), "is not a valid upload target tag")
}

type ImageRequest struct {
	ImgType  ImageType      `json:"image_type"`
	UpTarget []UploadTarget `json:"upload_targets"`
}

// ComposeRequest is used to submit a new compose to the store
type ComposeRequest struct {
	BlueprintName   string         `json:"blueprint_name"`
	ComposeID       uuid.UUID      `json:"uuid"`
	Distro          Distribution   `json:"distro"`
	Arch            Architecture   `json:"arch"`
	RequestedImages []ImageRequest `json:"requested_images"`
}
