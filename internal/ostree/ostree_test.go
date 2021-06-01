package ostree

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOstreeResolveRef(t *testing.T) {
	goodRef := "5330bb1b8820944567f519de66ad6354c729b6b490dea1c5a7ba320c9f147c58"
	badRef := "<html>not a ref</html>"

	handler := http.NewServeMux()
	handler.HandleFunc("/refs/heads/rhel/8/x86_64/edge", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	handler.HandleFunc("/refs/heads/test_forbidden", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "", http.StatusForbidden)
	})
	handler.HandleFunc("/refs/heads/get_bad_ref", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, badRef)
	})

	handler.HandleFunc("/refs/heads/test_redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/refs/heads/valid/ostree/ref", http.StatusFound)
	})
	handler.HandleFunc("/refs/heads/valid/ostree/ref", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, goodRef)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	type input struct {
		location string
		ref      string
	}
	validCases := map[input]string{
		{srv.URL, "test_redir"}:       goodRef,
		{srv.URL, "valid/ostree/ref"}: goodRef,
	}
	for in, expOut := range validCases {
		out, err := ResolveRef(in.location, in.ref)
		assert.NoError(t, err)
		assert.Equal(t, expOut, out)
	}

	errCases := map[input]string{
		{"not-a-url", "a-bad-ref"}:             "Get \"not-a-url/refs/heads/a-bad-ref\": unsupported protocol scheme \"\"",
		{"http://0.0.0.0:10/repo", "whatever"}: "Get \"http://0.0.0.0:10/repo/refs/heads/whatever\": dial tcp 0.0.0.0:10: connect: connection refused",
		{srv.URL, "rhel/8/x86_64/edge"}:        fmt.Sprintf("ostree repository \"%s/refs/heads/rhel/8/x86_64/edge\" returned status: 404 Not Found", srv.URL),
		{srv.URL, "test_forbidden"}:            fmt.Sprintf("ostree repository \"%s/refs/heads/test_forbidden\" returned status: 403 Forbidden", srv.URL),
		{srv.URL, "get_bad_ref"}:               fmt.Sprintf("ostree repository \"%s/refs/heads/get_bad_ref\" returned invalid reference", srv.URL),
	}
	for in, expMsg := range errCases {
		_, err := ResolveRef(in.location, in.ref)
		assert.EqualError(t, err, expMsg)
	}
}
