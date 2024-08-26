package worker_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
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

// proxy is a simple http-only proxy implementation.
// Don't use it in production. Also don't use it for https.
type proxy struct {
	// number of calls that were made through the proxy
	calls                  int
	paths                  []string
	registrationSuccessful bool
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	p.calls++

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

	req.Body = io.NopCloser(strings.NewReader(bodyString))

	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	p.paths = append(p.paths, fmt.Sprintf("%d: %s (%s) Body: %s (Response: %s)", p.calls, req.URL.Path, req.Method, strings.TrimSpace(bodyString), resp.Status))

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
