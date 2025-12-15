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
	"strings"
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
	baseURL, baseBuildDir, loggerHook := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	restore := main.MockOsbuildBinary(t, fmt.Sprintf(`#!/bin/sh -e
# make sure the monitor is setup correctly
>&3 echo '^^{"message": "osbuild-stage-message 1"}'
>&3 echo '^^{"message": "osbuild-stage-message 2"}'
>&3 echo '^^{"message": "osbuild-stage-message 3"}'

# echo our inputs for the test to validate
echo osbuild $@
echo ---
# stdin
cat -
echo

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

	// check that we get the monitor output of osbuild streamed to us
	expectedMonitorContent := `^^{"message": "osbuild-stage-message 1"}
^^{"message": "osbuild-stage-message 2"}
^^{"message": "osbuild-stage-message 3"}
`
	reader := bufio.NewReader(rsp.Body)
	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, expectedMonitorContent, string(content))

	// check log too
	expectedContent := fmt.Sprintf(`osbuild --store %[1]s/build/osbuild-store --output-directory %[1]s/build/output --cache-max-size=21474836480 - --export tree --monitor=JSONSeqMonitor --monitor-fd=3 --json
---
{"fake": "manifest"}
`, baseBuildDir)
	logFileContent, err := os.ReadFile(filepath.Join(baseBuildDir, "build/output/osbuild-result.json"))
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(logFileContent))
	// check that the "store" dir got created
	stat, err := os.Stat(filepath.Join(baseBuildDir, "build/osbuild-store"))
	assert.NoError(t, err)
	assert.True(t, stat.IsDir())

	// ensure tar is not generating any warnings
	for _, entry := range loggerHook.Entries {
		assert.NotContains(t, entry.Message, "unexpected tar output")
	}

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
	expected := []string{"output/",
		"output/image/",
		"output/image/disk.img",
		"output/osbuild-result.json",
	}
	actual := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
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
	assert.Equal(t, "cannot run osbuild: exit status 23", string(content))

	// check that the result is an error and we get the log
	endpoint = baseURL + "api/v1/result/image/disk.img"
	rsp, err = http.Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, rsp.StatusCode)
	reader = bufio.NewReader(rsp.Body)
	content, err = io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, `build failed
err on stdout
`, string(content))
}

func TestBuildStreamsOutput(t *testing.T) {
	baseURL, baseBuildDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/build"

	restore := main.MockOsbuildBinary(t, fmt.Sprintf(`#!/bin/sh -e
for i in $(seq 3); do
   # generate the exact timestamp of the output line
   >&3 echo "line-$i: $(date  +'%%s.%%N')"
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

func TestBuildErrorHandlingResult(t *testing.T) {
	restore := main.MockOsbuildBinary(t, `#!/bin/sh

# not creating an output dir, this will lead to errors from the "tar"
# step
echo osbuild failure on stdout
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
	assert.Contains(t, string(body), "unable to move result file to output directory:")
	assert.Contains(t, string(body), "osbuild failure on stdout")
	assert.Contains(t, loggerHook.LastEntry().Message, "unable to move result file to output directory:")
}
