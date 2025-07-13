package hashutil

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// Sha256sum() is a convenience wrapper to generate
// the sha256 hex digest of a file. The hash is the
// same as from the sha256sum util.
func Sha256sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
