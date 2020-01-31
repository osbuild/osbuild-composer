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

func TestRandomSHA1String(t *testing.T) {
	hash, err := randomSHA1String()
	if err != nil {
		t.Fatalf("RandomSHA1String failed: %s", err)
	}
	if len(hash) != 40 {
		t.Fatalf("RandomSHA1String failed: hash is not 40 characters")
	}
}
