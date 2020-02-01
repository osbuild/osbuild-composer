package store

import (
	"testing"
)

func TestRandomSHA1String(t *testing.T) {
	hash, err := randomSHA1String()
	if err != nil {
		t.Fatalf("RandomSHA1String failed: %s", err)
	}
	if len(hash) != 40 {
		t.Fatalf("RandomSHA1String failed: hash is not 40 characters")
	}
}
