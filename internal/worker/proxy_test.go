package worker_test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

// details of the request
// (not testing the response here)
type callDetails struct {
	path   string
	method string
	body   string
}

// helper for "equality" in tests
// as the expected body is only to be contained in the actual request
// and path can be regex in the "expected" struct
func (expected callDetails) Equals(t *testing.T, actual callDetails) bool {
	if !(strings.Contains(actual.body, expected.body) && expected.method == actual.method) {
		return false
	}

	re, err := regexp.Compile(expected.path)
	require.NoError(t, err)

	if err != nil {
		return false
	}

	// Check if other.Field2 matches the regex stored in m.Field2
	return re.MatchString(actual.path)
}

// proxy is a simple http-only proxy implementation.
// Don't use it in production. Also don't use it for https.
type proxy struct {
	// number of calls that were made through the proxy
	calls                  []callDetails
	paths                  []string
	registrationSuccessful bool
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	p.calls = append(p.calls, callDetails{path: req.URL.Path, method: req.Method})

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(wr, msg, http.StatusBadRequest)
		return
	}

	client := &http.Client{}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	delHopHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(wr, "Cant' read request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	bodyString := string(bodyBytes)
	p.calls[len(p.calls)-1].body = bodyString

	req.Body = io.NopCloser(strings.NewReader(bodyString))

	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	p.paths = append(p.paths, fmt.Sprintf("%d: %s (%s) Body: %s (Response: %s)", len(p.calls), req.URL.Path, req.Method, strings.TrimSpace(bodyString), resp.Status))

	if req.URL.Path == "/api/image-builder-worker/v1/workers" &&
		req.Method == "POST" &&
		resp.StatusCode == 201 {
		p.registrationSuccessful = true
	}

	delHopHeaders(resp.Header)

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(wr, resp.Body)
}
