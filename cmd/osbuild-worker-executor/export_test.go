package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var (
	Run = run

	HandleIncludedSources = handleIncludedSources
)

func MockOsbuildBinary(t *testing.T, new string) (restore func()) {
	t.Helper()

	tmpdir := t.TempDir()
	/* #nosec G306 */
	if err := os.WriteFile(filepath.Join(tmpdir, "osbuild"), []byte(new), 0755); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s:%s", tmpdir, path))

	return func() {
		os.Setenv("PATH", path)
	}
}
