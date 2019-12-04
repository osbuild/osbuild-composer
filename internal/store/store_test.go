package store

import (
	"testing"
)

func TestBumpVersion(t *testing.T) {
	cases := []struct {
		Version  string
		Expected string
	}{
		{"", ""},
		{"0", "0.0.1"},
		{"0.0", "0.0.1"},
		{"0.0.0", "0.0.1"},
		{"2.1.3", "2.1.4"},

		// don't touch invalid version strings
		{"0.0.0.0", "0.0.0.0"},
		{"0.a.0", "0.a.0"},
		{"foo", "foo"},
	}

	for _, c := range cases {
		result := bumpVersion(c.Version)
		if result != c.Expected {
			t.Errorf("bumpVersion(%#v) is expected to return %#v, but instead returned %#v", c.Version, c.Expected, result)
		}
	}
}
