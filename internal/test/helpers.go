package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func externalRequest(method, path, body string) *http.Response {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/weldr/api.socket")
			},
		},
	}

	req, err := http.NewRequest(method, "http://localhost"+path, bytes.NewReader([]byte(body)))
	if err != nil {
		panic(err)
	}

	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	return resp
}

func internalRequest(api http.Handler, method, path, body string) *http.Response {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	return resp.Result()
}

func SendHTTP(api http.Handler, external bool, method, path, body string) *http.Response {
	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		if !external {
			return nil
		}
		return externalRequest(method, path, body)
	} else {
		return internalRequest(api, method, path, body)
	}
}

// this function serves to drop fields that shouldn't be tested from the unmarshalled json objects
func dropFields(obj interface{}, fields ...string) {
	switch v := obj.(type) {
	// if the interface type is a map attempt to delete the fields
	case map[string]interface{}:
		for i, field := range fields {
			if _, ok := v[field]; ok {
				delete(v, field)
				// if the field is found remove it from the fields slice
				if len(fields) < i-1 {
					fields = append(fields[:i], fields[i+1:]...)
				} else {
					fields = fields[:i]
				}
			}
		}
		// call dropFields on the remaining elements since they may contain a map containing the field
		for _, val := range v {
			dropFields(val, fields...)
		}
	// if the type is a list of interfaces call dropFields on each interface
	case []interface{}:
		for _, element := range v {
			dropFields(element, fields...)
		}
	default:
		return
	}
}

func TestRoute(t *testing.T, api http.Handler, external bool, method, path, body string, expectedStatus int, expectedJSON string, ignoreFields ...string) {
	t.Helper()

	resp := SendHTTP(api, external, method, path, body)
	if resp == nil {
		t.Skip("This test is for internal testing only")
	}

	replyJSON, err := ioutil.ReadAll(resp.Body)
	require.NoErrorf(t, err, "%s: could not read response body", path)

	assert.Equalf(t, expectedStatus, resp.StatusCode, "SendHTTP failed for path %s: %v", path, string(replyJSON))

	if expectedJSON == "" {
		require.Lenf(t, replyJSON, 0, "%s: expected no response body, but got:\n%s", path, replyJSON)
	}

	if expectedJSON == "?" {
		return
	}

	var reply, expected interface{}
	err = json.Unmarshal(replyJSON, &reply)
	require.NoErrorf(t, err, "%s: json.Unmarshal failed for\n%s", path, string(replyJSON))

	if expectedJSON == "*" {
		return
	}

	err = json.Unmarshal([]byte(expectedJSON), &expected)
	require.NoErrorf(t, err, "%s: expected JSON is invalid", path)

	dropFields(reply, ignoreFields...)
	dropFields(expected, ignoreFields...)

	require.Equal(t, expected, reply)
}

func TestTOMLRoute(t *testing.T, api http.Handler, external bool, method, path, body string, expectedStatus int, expectedTOML string, ignoreFields ...string) {
	t.Helper()

	resp := SendHTTP(api, external, method, path, body)
	if resp == nil {
		t.Skip("This test is for internal testing only")
	}

	replyTOML, err := ioutil.ReadAll(resp.Body)
	require.NoErrorf(t, err, "%s: could not read response body", path)

	assert.Equalf(t, expectedStatus, resp.StatusCode, "SendHTTP failed for path %s: %v", path, string(replyTOML))

	if expectedTOML == "" {
		require.Lenf(t, replyTOML, 0, "%s: expected no response body, but got:\n%s", path, replyTOML)
	}

	var reply, expected interface{}
	err = toml.Unmarshal(replyTOML, &reply)
	require.NoErrorf(t, err, "%s: json.Unmarshal failed for\n%s", path, string(replyTOML))

	if expectedTOML == "*" {
		return
	}

	err = toml.Unmarshal([]byte(expectedTOML), &expected)
	require.NoErrorf(t, err, "%s: expected TOML is invalid", path)

	dropFields(reply, ignoreFields...)
	dropFields(expected, ignoreFields...)

	require.Equal(t, expected, reply)
}

func TestNonJsonRoute(t *testing.T, api http.Handler, external bool, method, path, body string, expectedStatus int, expectedResponse string) {
	response := SendHTTP(api, external, method, path, body)
	assert.Equalf(t, expectedStatus, response.StatusCode, "%s: status mismatch", path)

	responseBodyBytes, err := ioutil.ReadAll(response.Body)
	require.NoErrorf(t, err, "%s: could not read response body", path)

	responseBody := string(responseBodyBytes)
	require.Equalf(t, expectedResponse, responseBody, "%s: body mismatch", path)
}

func IgnoreDates() cmp.Option {
	return cmp.Comparer(func(a, b time.Time) bool { return true })
}

func IgnoreUuids() cmp.Option {
	return cmp.Comparer(func(a, b uuid.UUID) bool { return true })
}

func Ignore(what string) cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool { return p.String() == what }, cmp.Ignore())
}

// CompareImageType considers two image type objects equal if and only if the names of their distro/arch/imagetype
// are. The thinking is that the objects are static, and resolving by these three keys should always give equivalent
// objects. Whether we actually have object equality, is an implementation detail, so we don't want to rely on that.
func CompareImageTypes() cmp.Option {
	return cmp.Comparer(func(x, y distro.ImageType) bool {
		return x.Name() == y.Name() &&
			x.Arch().Name() == y.Arch().Name() &&
			x.Arch().Distro().Name() == y.Arch().Distro().Name()
	})
}

// Create a temporary repository
func SetUpTemporaryRepository() (string, error) {
	dir, err := ioutil.TempDir("/tmp", "osbuild-composer-test-")
	if err != nil {
		return "", err
	}
	cmd := exec.Command("createrepo_c", path.Join(dir))
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}
	return dir, nil
}

// Remove the temporary repository
func TearDownTemporaryRepository(dir string) error {
	return os.RemoveAll(dir)
}
