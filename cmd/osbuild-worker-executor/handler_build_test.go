package main_test

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker-executor"
)

func TestBuildMustPOST(t *testing.T) {
	baseURL, _, loggerHook := runTestServer(t)

	endpoint := baseURL + "api/v1/build"
	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, 405, rsp.StatusCode)
	assert.Equal(t, "handlerBuild called on /api/v1/build", loggerHook.LastEntry().Message)
}

func writeToTar(atar *tar.Writer, name, content string) error {
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := atar.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := atar.Write([]byte(content))
	return err
}

func TestBuildChecksContentType(t *testing.T) {
	baseURL, _, _ := runTestServer(t)

	endpoint := baseURL + "api/v1/build"
	rsp, err := http.Post(endpoint, "random/encoding", nil)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusUnsupportedMediaType, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "Content-Type must be [application/x-tar], got random/encoding\n", string(body))
}

func makeTestPost(t *testing.T, controlJSON, manifestJSON string) *bytes.Buffer {
	buf := bytes.NewBuffer(nil)
	archive := tar.NewWriter(buf)
	err := writeToTar(archive, "control.json", controlJSON)
	assert.NoError(t, err)
	err = writeToTar(archive, "manifest.json", manifestJSON)
	assert.NoError(t, err)
	// for now we assume we get validated data, for files we could
	// trivially validate on the fly but for containers that is
	// harder
	for _, dir := range []string{"osbuild-store/", "osbuild-store/sources", "osbuild-store/sources/org.osbuild.files"} {
		err = archive.WriteHeader(&tar.Header{
			Name:     dir,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})
		assert.NoError(t, err)
	}
	err = writeToTar(archive, "osbuild-store/sources/org.osbuild.files/sha256:ff800c5263b915d8a0776be5620575df2d478332ad35e8dd18def6a8c720f9c7", "random-data")
	assert.NoError(t, err)
	err = writeToTar(archive, "osbuild-store/sources/org.osbuild.files/sha256:aabbcc5263b915d8a0776be5620575df2d478332ad35e8dd18def6a8c720f9c7", "other-data")
	assert.NoError(t, err)
	return buf
}

func TestBuildIntegration(t *testing.T) {
	baseURL, baseBuildDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	// osbuild is called with --export tree and then the manifest.json
	restore := main.MockOsbuildBinary(t, fmt.Sprintf(`#!/bin/sh -e
# echo our inputs for the test to validate
echo fake-osbuild "$1" "$2" "$3" "$4" "$5" "$6" "$7"
echo ---
cat "$8"

test "$MY" = "env"

# simulate output
mkdir -p %[1]s/build/output/image
echo "fake-build-result" > %[1]s/build/output/image/disk.img
`, baseBuildDir))
	defer restore()

	buf := makeTestPost(t, `{"exports": ["tree"], "environments": ["MY=env"]}`, `{"fake": "manifest"}`)
	rsp, err := http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	defer func() { _, _ = io.ReadAll(rsp.Body) }()
	defer rsp.Body.Close()

	assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	reader := bufio.NewReader(rsp.Body)

	// check that we get the output of osbuild streamed to us
	expectedContent := fmt.Sprintf(`fake-osbuild --export tree --output-dir %[1]s/build/output --store %[1]s/build/osbuild-store --json
---
{"fake": "manifest"}`, baseBuildDir)
	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))
	// check log too
	logFileContent, err := os.ReadFile(filepath.Join(baseBuildDir, "build/build.log"))
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(logFileContent))
	// check that the "store" dir got created
	stat, err := os.Stat(filepath.Join(baseBuildDir, "build/osbuild-store"))
	assert.NoError(t, err)
	assert.True(t, stat.IsDir())

	// now get the result
	endpoint = baseURL + "api/v1/result/image/disk.img"
	rsp, err = http.Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "fake-build-result\n", string(body))

	// check that the output tarball has the disk in it
	endpoint = baseURL + "api/v1/result/output.tar"
	rsp, err = http.Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err = io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	tarPath := filepath.Join(baseBuildDir, "output.tar")
	assert.NoError(t, os.WriteFile(tarPath, body, 0644))
	cmd := exec.Command("tar", "-tf", tarPath)
	out, err := cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, "output/\noutput/image/\noutput/image/disk.img\n", string(out))
}

func TestBuildErrorsForMultipleBuilds(t *testing.T) {
	baseURL, buildDir, loggerHook := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	restore := main.MockOsbuildBinary(t, fmt.Sprintf(`#!/bin/sh

mkdir -p %[1]s/build/output/image
echo "fake-build-result" > %[1]s/build/output/image/disk.img
`, buildDir))
	defer restore()

	buf := makeTestPost(t, `{"exports": ["tree"]}`, `{"fake": "manifest"}`)
	rsp, err := http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	defer func() { _, _ = io.ReadAll(rsp.Body) }()
	defer rsp.Body.Close()

	buf = makeTestPost(t, `{"exports": ["tree"]}`, `{"fake": "manifest"}`)
	rsp, err = http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusConflict, rsp.StatusCode)
	assert.Equal(t, main.ErrAlreadyBuilding.Error(), loggerHook.LastEntry().Message)
}

func TestHandleIncludedSourcesUnclean(t *testing.T) {
	tmpdir := t.TempDir()

	buf := bytes.NewBuffer(nil)
	atar := tar.NewWriter(buf)
	err := writeToTar(atar, "osbuild-store/../../etc/passwd", "some-content")
	assert.NoError(t, err)

	err = main.HandleIncludedSources(tar.NewReader(buf), tmpdir)
	assert.EqualError(t, err, "name not clean: ../etc/passwd != osbuild-store/../../etc/passwd")
}

func TestHandleIncludedSourcesNotFromStore(t *testing.T) {
	tmpdir := t.TempDir()

	buf := bytes.NewBuffer(nil)
	atar := tar.NewWriter(buf)
	err := writeToTar(atar, "not-store", "some-content")
	assert.NoError(t, err)

	err = main.HandleIncludedSources(tar.NewReader(buf), tmpdir)
	assert.EqualError(t, err, "expected osbuild-store/ prefix, got not-store")
}

func TestHandleIncludedSourcesBadTypes(t *testing.T) {
	tmpdir := t.TempDir()

	for _, badType := range []byte{tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo} {
		buf := bytes.NewBuffer(nil)
		atar := tar.NewWriter(buf)
		err := atar.WriteHeader(&tar.Header{
			Name:     "osbuild-store/bad-type",
			Typeflag: badType,
		})
		assert.NoError(t, err)

		err = main.HandleIncludedSources(tar.NewReader(buf), tmpdir)
		assert.EqualError(t, err, fmt.Sprintf("unsupported tar type %v", badType))
	}
}

func TestBuildIntegrationOsbuildError(t *testing.T) {
	baseURL, _, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	// osbuild is called with --export tree and then the manifest.json
	restore := main.MockOsbuildBinary(t, `#!/bin/sh -e
# simulate failure
echo "err on stdout"
>&2 echo "err on stderr"
exit 23
`)
	defer restore()

	buf := makeTestPost(t, `{"exports": ["tree"], "environments": ["MY=env"]}`, `{"fake": "manifest"}`)
	rsp, err := http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	defer func() { _, _ = io.ReadAll(rsp.Body) }()
	defer rsp.Body.Close()

	assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	reader := bufio.NewReader(rsp.Body)
	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	expectedContent := `err on stdout
err on stderr
cannot run osbuild: exit status 23`
	assert.Equal(t, expectedContent, string(content))

	// check that the result is an error and we get the log
	endpoint = baseURL + "api/v1/result/image/disk.img"
	rsp, err = http.Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, rsp.StatusCode)
	reader = bufio.NewReader(rsp.Body)
	content, err = io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, "build failed\n"+expectedContent, string(content))
}

func TestBuildStreamsOutput(t *testing.T) {
	baseURL, baseBuildDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	restore := main.MockOsbuildBinary(t, fmt.Sprintf(`#!/bin/sh -e
for i in $(seq 3); do
   # generate the exact timestamp of the output line
   echo "line-$i: $(date  +'%%s.%%N')"
   sleep 0.2
done

# simulate output
mkdir -p %[1]s/build/output/image
echo "fake-build-result" > %[1]s/build/output/image/disk.img
`, baseBuildDir))
	defer restore()

	buf := makeTestPost(t, `{"exports": ["tree"], "environments": ["MY=env"]}`, `{"fake": "manifest"}`)
	rsp, err := http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	defer func() { _, _ = io.ReadAll(rsp.Body) }()
	defer rsp.Body.Close()

	assert.Equal(t, http.StatusCreated, rsp.StatusCode)
	reader := bufio.NewReader(rsp.Body)
	var lineno, seconds, nano int64
	for i := 1; i <= 3; i++ {
		line, err := reader.ReadString('\n')
		assert.NoError(t, err)
		// the out contains when it was generated
		_, err = fmt.Sscanf(line, "line-%d: %d.%d\n", &lineno, &seconds, &nano)
		assert.NoError(t, err)
		timeSinceOutput := time.Since(time.Unix(seconds, nano))
		// we expect lines to appear right away, for really slow VMs
		// we give a grace time of 200ms (which should be plenty and
		// is also a bit arbitrary)
		assert.True(t, timeSinceOutput < 200*time.Millisecond, fmt.Sprintf("output did not arrive in the expected time interval, delay: %v", timeSinceOutput))
	}
}

func TestBuildErrorHandlingTar(t *testing.T) {
	restore := main.MockOsbuildBinary(t, `#!/bin/sh

# not creating an output dir, this will lead to errors from the "tar"
# step
`)
	defer restore()

	baseURL, _, loggerHook := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	buf := makeTestPost(t, `{"exports": ["tree"]}`, `{"fake": "manifest"}`)
	rsp, err := http.Post(endpoint, "application/x-tar", buf)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, rsp.StatusCode, http.StatusCreated)

	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "cannot tar output directory:")
	assert.Contains(t, loggerHook.LastEntry().Message, "cannot tar output directory:")
}
