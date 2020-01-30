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
