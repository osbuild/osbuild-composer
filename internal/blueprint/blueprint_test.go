package blueprint

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestDeepCopy(t *testing.T) {
	bpOrig := Blueprint{
		Name:        "deepcopy-test",
		Description: "Testing DeepCopy function",
		Version:     "0.0.1",
		Packages: []Package{
			{Name: "dep-package1", Version: "*"}},
		Modules: []Package{
			{Name: "dep-package2", Version: "*"}},
	}

	bpCopy, err := bpOrig.DeepCopy()
	if err != nil {
		t.Fatalf("Blueprint.DeepCopy failure: %s", err.Error())
	}
	if diff := cmp.Diff(bpOrig, bpCopy); diff != "" {
		t.Fatalf("Blueprint.DeepCopy is different from original\ndiff: %s", diff)
	}

	// Modify the copy
	bpCopy.Packages[0].Version = "1.2.3"
	if bpOrig.Packages[0].Version != "*" {
		t.Fatalf("Blueprint.DeepCopy failed, original modified")
	}

	// Modify the original
	bpOrig.Packages[0].Version = "42.0"
	if bpCopy.Packages[0].Version != "1.2.3" {
		t.Fatalf("Blueprint.DeepCopy failed, copy modified")
	}
}

func TestBlueprintInitialize(t *testing.T) {
	cases := []struct {
		NewBlueprint  Blueprint
		ExpectedError bool
	}{
		{Blueprint{Name: "bp-test-1", Description: "Empty version", Version: ""}, true},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 1", Version: "0"}, true},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 2", Version: "0.0"}, true},
		{Blueprint{Name: "bp-test-3", Description: "Invalid version 3", Version: "0.0.0.0"}, true},
		{Blueprint{Name: "bp-test-4", Description: "Invalid version 4", Version: "0.a.0"}, true},
		{Blueprint{Name: "bp-test-5", Description: "Invalid version 5", Version: "foo"}, true},
		{Blueprint{Name: "bp-test-7", Description: "Zero version", Version: "0.0.0"}, false},
		{Blueprint{Name: "bp-test-8", Description: "X.Y.Z version", Version: "2.1.3"}, false},
	}

	for _, c := range cases {
		bp := c.NewBlueprint
		err := bp.Initialize()
		if (err != nil) != c.ExpectedError {
			t.Errorf("Initialize(%#v) returned an unexpected error: %s", c.NewBlueprint, err.Error())
		}
	}
}

func TestBumpVersion(t *testing.T) {
	cases := []struct {
		NewBlueprint    Blueprint
		OldVersion      string
		ExpectedVersion string
	}{
		{Blueprint{Name: "bp-test-1", Description: "Empty version", Version: "0.0.1"}, "", "0.0.1"},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 1", Version: "0.0.1"}, "0", "0.0.1"},
		{Blueprint{Name: "bp-test-3", Description: "Invalid version 2", Version: "0.0.1"}, "0.0.0.0", "0.0.1"},
		{Blueprint{Name: "bp-test-4", Description: "Invalid version 3", Version: "0.0.1"}, "0.a.0", "0.0.1"},
		{Blueprint{Name: "bp-test-5", Description: "Invalid version 4", Version: "0.0.1"}, "foo", "0.0.1"},
		{Blueprint{Name: "bp-test-6", Description: "Invalid version 5", Version: "0.0.1"}, "0.0", "0.0.1"},
		{Blueprint{Name: "bp-test-8", Description: "Same version", Version: "4.2.0"}, "4.2.0", "4.2.1"},
	}

	for _, c := range cases {
		bp := c.NewBlueprint
		bp.Initialize()

		bp.BumpVersion(c.OldVersion)
		if bp.Version != c.ExpectedVersion {
			t.Errorf("BumpVersion(%#v) is expected to return %#v, but instead returned %#v", c.OldVersion, c.ExpectedVersion, bp.Version)
		}
	}
}
