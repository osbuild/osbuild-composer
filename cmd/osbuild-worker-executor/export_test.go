package main

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	Run = run

	HandleIncludedSources = handleIncludedSources
)

func MockUnixSethostname(new func([]byte) error) (restore func()) {
	saved := unixSethostname
	unixSethostname = new
	return func() {
		unixSethostname = saved
	}
}

func MockOsbuildBinary(t *testing.T, new string) (restore func()) {
	t.Helper()

	saved := osbuildBinary

	tmpdir := t.TempDir()
	osbuildBinary = filepath.Join(tmpdir, "fake-osbuild")
	/* #nosec G306 */
	if err := os.WriteFile(osbuildBinary, []byte(new), 0755); err != nil {
		t.Fatal(err)
	}

	return func() {
		osbuildBinary = saved
	}
}
