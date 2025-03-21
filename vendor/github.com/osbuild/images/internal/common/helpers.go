package common

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
)

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// IsStringInSortedSlice returns true if the string is present, false if not
// slice must be sorted
func IsStringInSortedSlice(slice []string, s string) bool {
	i := sort.SearchStrings(slice, s)
	if i < len(slice) && slice[i] == s {
		return true
	}
	return false
}

// NopSeekCloser returns an io.ReadSeekCloser with a no-op Close method
// wrapping the provided io.ReadSeeker r.
func NopSeekCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return nopSeekCloser{r}
}

type nopSeekCloser struct {
	io.ReadSeeker
}

func (nopSeekCloser) Close() error { return nil }

// MountUnitNameFor returns the escaped name of the mount unit for a given
// mountpoint by calling:
//
//	systemd-escape --path --suffix=mount "mountpoint"
func MountUnitNameFor(mountpoint string) (string, error) {
	cmd := exec.Command("systemd-escape", "--path", "--suffix=mount", mountpoint)
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("systemd-escape call failed: %s", ExecError(err))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// ExecError handles the error from an exec.Command().Output() call. It returns
// a formatted error that includes StdErr when the error is of type
// exec.ExitError.
func ExecError(err error) error {
	if err, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s [%w]", bytes.TrimSpace(err.Stderr), err)
	}
	return err
}

// Must() can be used to shortcut all `NewT() (T, err)` constructors.
// It will panic if an error is passed.
func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}
