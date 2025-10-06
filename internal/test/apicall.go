package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// APICall is a small function object for testing HTTP APIs
type APICall struct {
	// http.Handler to run the call against
	Handler http.Handler

	// HTTP method, e.g. http.MethodPatch
	Method string

	// Request Path
	Path string

	// Request body. If nil, an empty body is sent
	RequestBody RequestBody

	// Request header. If nil, default header is sent
	Header http.Header

	// Request context. If nil, default context is used
	Context context.Context

	// Status that's expected to be received. If set to 0, the status is not checked
	ExpectedStatus int

	// Validator for the response body. If set to nil, the body is not validated
	ExpectedBody BodyValidator
}

// Do performs the request as defined in the APICall struct.
//
// If any errors occur when doing the request, or any of the validators fail, t.FailNow() is called
// Note that HTTP error status is not checked if ExpectedStatus == 0
//
// The result of the HTTP call is returned
func (a APICall) Do(t *testing.T) APICallResult {
	t.Helper()

	var bodyReader io.Reader
	if a.RequestBody != nil {
		bodyReader = bytes.NewReader(a.RequestBody.Body())
	}

	req := httptest.NewRequest(a.Method, a.Path, bodyReader)

	if a.Context != nil {
		req = req.WithContext(a.Context)
	}

	req.Header = a.Header
	if req.Header == nil {
		req.Header = http.Header{}
	}

	if a.RequestBody != nil && a.RequestBody.ContentType() != "" {
		req.Header.Set("Content-Type", a.RequestBody.ContentType())
	}
	respRecorder := httptest.NewRecorder()
	a.Handler.ServeHTTP(respRecorder, req)
	resp := respRecorder.Result()

	body, err := io.ReadAll(resp.Body)
	require.NoErrorf(t, err, "%s: could not read response body", a.Path)

	if a.ExpectedStatus != 0 {
		assert.Equalf(t, a.ExpectedStatus, resp.StatusCode, "%s: SendHTTP failed for path; body: %s", a.Path, string(body))
	}
	if a.ExpectedBody != nil {
		err = a.ExpectedBody.Validate(body)
		require.NoError(t, err, "%s: cannot validate response body", a.Path)
	}

	return APICallResult{
		Body:       body,
		StatusCode: resp.StatusCode,
	}
}

// APICallResult holds a parsed response for an APICall
type APICallResult struct {
	// Full body as read from the server
	Body []byte

	// Status code returned from the server
	StatusCode int
}
