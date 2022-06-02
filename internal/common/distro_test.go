package common

import (
	"reflect"
	"strings"
	"testing"
)

func TestOSRelease(t *testing.T) {
	var cases = []struct {
		Input     string
		OSRelease map[string]string
	}{
		{
			``,
			map[string]string{},
		},
		{
			`NAME=Fedora
VERSION="30 (Workstation Edition)"
ID=fedora
VERSION_ID=30
VERSION_CODENAME=""
PLATFORM_ID="platform:f30"
PRETTY_NAME="Fedora 30 (Workstation Edition)"
VARIANT="Workstation Edition"
VARIANT_ID=workstation`,
			map[string]string{
				"NAME":             "Fedora",
				"VERSION":          "30 (Workstation Edition)",
				"ID":               "fedora",
				"VERSION_ID":       "30",
				"VERSION_CODENAME": "",
				"PLATFORM_ID":      "platform:f30",
				"PRETTY_NAME":      "Fedora 30 (Workstation Edition)",
				"VARIANT":          "Workstation Edition",
				"VARIANT_ID":       "workstation",
			},
		},
	}

	for i, c := range cases {
		r := strings.NewReader(c.Input)

		osrelease, err := readOSRelease(r)
		if err != nil {
			t.Fatalf("%d: readOSRelease: %v", i, err)
		}

		if !reflect.DeepEqual(osrelease, c.OSRelease) {
			t.Fatalf("%d: readOSRelease returned unexpected result: %#v", i, osrelease)
		}
	}
}
