package osbuildexecutor_test

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
)

func makeTestTarfile(t *testing.T, content map[*tar.Header]string) string {
	tmpdir := t.TempDir()
	testTarPath := filepath.Join(tmpdir, "test.tar")
	f, err := os.Create(testTarPath)
	assert.NoError(t, err)
	defer f.Close()

	atar := tar.NewWriter(f)
	for hdr, fcnt := range content {
		if hdr.Mode == 0 {
			hdr.Mode = 0644
		}
		hdr.Size = int64(len(fcnt))

		err := atar.WriteHeader(hdr)
		assert.NoError(t, err)
		_, err = atar.Write([]byte(fcnt))
		assert.NoError(t, err)
	}

	return testTarPath
}

func TestValidateOutputArchiveHappy(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "file1"}:        "some content",
		&tar.Header{Name: "path/to/file"}: "other content",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.NoError(t, err)
}

func TestValidateOutputArchiveSadDotDot(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "file1/.."}: "some content",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "file1/.." not clean, got "." after cleaning`)
}

func TestValidateOutputArchiveSadAbsolutePath(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "/file1"}: "some content",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "/file1" must not start with an absolute path`)
}

func TestValidateOutputArchiveSadBadType(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "dev/sda", Typeflag: tar.TypeBlock}: "",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "dev/sda" must be a file/dir, is header type '4'`)
}

func TestValidateOutputArchiveSadExecutable(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "exe", Mode: 0755}: "#!/bin/sh p0wned",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "exe" must not be executable (is mode 0755)`)
}
