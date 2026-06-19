//go:build !cgo

//
// fallback implementation of "crypt" for cross building

package crypt

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func crypt(pass, salt string) (string, error) {
	// we could extract the "hash-type" here and pass it to
	// openssl instead of hardcoding -6 but lets do that once we
	// actually move away from sha512crypt
	if !strings.HasPrefix(salt, "$6$") {
		return "", fmt.Errorf("only crypt type SHA512 supported, got %q", salt)
	}
	cmd := exec.Command(
		"openssl", "passwd", "-6",
		// strip the $6$
		"-salt", salt[3:],
		"-stdin",
	)
	cmd.Stdin = bytes.NewBufferString(pass)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cannot generate password: %v, output:%s\n", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
