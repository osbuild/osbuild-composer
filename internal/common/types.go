package common

import (
	"encoding/json"
	"fmt"
)

// CustomJsonConversionError is thrown when parsing strings into enumerations
type CustomJsonConversionError struct {
	reason string
}

// Error returns the error as a string
func (err *CustomJsonConversionError) Error() string {
	return err.reason
}

// CustomTypeError is thrown when parsing strings into enumerations
type CustomTypeError struct {
	reason string
}

// Error returns the error as a string
func (err *CustomTypeError) Error() string {
	return err.reason
}

// Since Go has no generics, this is the only way to write common code for all the types present in this package.
// It uses weakly typed maps to convert between strings and integers which are then wrapped into type aliases.
// Specific implementations are bellow each type.
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

// See unmarshalHelper for explanation
func marshalHelper(input int, mapping map[string]int, errorMessage string) ([]byte, error) {
	for k, v := range mapping {
		if v == input {
			return json.Marshal(k)
		}
	}
	return nil, &CustomJsonConversionError{fmt.Sprintf("%d %s", input, errorMessage)}
}

// See unmarshalHelper for introduction. Converts between TypeAlias(int) and string
func toStringHelper(mapping map[string]int, tag int) (string, bool) {
	for k, v := range mapping {
		if v == tag {
			return k, true
		}
	}
	return "", false
}

type ImageType int

// NOTE: If you want to add more constants here, don't forget to add a mapping below
const (
	Azure ImageType = iota
	Aws
	LiveISO
	OpenStack
	Qcow2Generic
	Vmware
	RawFilesystem
	PartitionedDisk
	TarArchive
)

// getArchMapping is a helper function that defines the conversion from JSON string value
// to ImageType.
func getImageTypeMapping() map[string]int {
	mapping := map[string]int{
		"Azure":            int(Azure),
		"AWS":              int(Aws),
		"LiveISO":          int(LiveISO),
		"OpenStack":        int(OpenStack),
		"qcow2":            int(Qcow2Generic),
		"VMWare":           int(Vmware),
		"Raw-filesystem":   int(RawFilesystem),
		"Partitioned-disk": int(PartitionedDisk),
		"Tar":              int(TarArchive),
	}
	return mapping
}

// TODO: check the mapping here:
func getCompatImageTypeMapping() map[int]string {
	mapping := map[int]string{
		int(Azure):           "vhd",
		int(Aws):             "ami",
		int(LiveISO):         "liveiso",
		int(OpenStack):       "openstack",
		int(Qcow2Generic):    "qcow2",
		int(Vmware):          "vmdk",
		int(RawFilesystem):   "ext4-filesystem",
		int(PartitionedDisk): "partitioned-disk",
		int(TarArchive):      "tar",
	}
	return mapping
}

func (imgType *ImageType) UnmarshalJSON(data []byte) error {
	value, err := unmarshalHelper(data, " is not a valid JSON value", " is not a valid image type", getImageTypeMapping())
	if err != nil {
		return err
	}
	*imgType = ImageType(value)
	return nil
}

func (imgType ImageType) MarshalJSON() ([]byte, error) {
	return marshalHelper(int(imgType), getImageTypeMapping(), "is not a valid image type tag")
}

func (imgType ImageType) ToCompatString() (string, bool) {
	str, exists := getCompatImageTypeMapping()[int(imgType)]
	return str, exists
}

func ImageTypeFromCompatString(input string) (ImageType, bool) {
	for k, v := range getCompatImageTypeMapping() {
		if v == input {
			return ImageType(k), true
		}
	}
	return ImageType(999), false
}

func (imgType ImageType) ToString() (string, bool) {
	return toStringHelper(getImageTypeMapping(), int(imgType))
}
