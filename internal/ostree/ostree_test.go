package ostree

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/ostree/test_mtls_server"
	"github.com/osbuild/osbuild-composer/internal/rhsm"
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

	mTLSSrv, err := test_mtls_server.NewMTLSServer(handler)
	srv2 := mTLSSrv.Server
	require.NoError(t, err)
	defer srv2.Close()
	subs := &rhsm.Subscriptions{
		Consumer: &rhsm.ConsumerSecrets{
			ConsumerKey:  mTLSSrv.ClientKeyPath,
			ConsumerCert: mTLSSrv.ClientCrtPath,
		},
	}

	type srvConfig struct {
		Srv  *httptest.Server
		RHSM bool
		Subs *rhsm.Subscriptions
	}
	srvConfs := []srvConfig{
		srvConfig{
			Srv:  srv,
			RHSM: false,
			Subs: nil,
		},
		srvConfig{
			Srv:  srv2,
			RHSM: true,
			Subs: subs,
		},
	}

	type input struct {
		location string
		ref      string
	}

	for _, srvConf := range srvConfs {
		validCases := map[input]string{
			{srvConf.Srv.URL, "test_redir"}:       goodRef,
			{srvConf.Srv.URL, "valid/ostree/ref"}: goodRef,
		}
		for in, expOut := range validCases {
			out, err := ResolveRef(in.location, in.ref, srvConf.RHSM, srvConf.Subs, &mTLSSrv.CAPath)
			assert.NoError(t, err)
			assert.Equal(t, expOut, out)
		}

		errCases := map[input]string{
			{"not-a-url", "a-bad-ref"}:              "error sending request to ostree repository \"not-a-url/refs/heads/a-bad-ref\": Get \"not-a-url/refs/heads/a-bad-ref\": unsupported protocol scheme \"\"",
			{"http://0.0.0.0:10/repo", "whatever"}:  "error sending request to ostree repository \"http://0.0.0.0:10/repo/refs/heads/whatever\": Get \"http://0.0.0.0:10/repo/refs/heads/whatever\": dial tcp 0.0.0.0:10: connect: connection refused",
			{srvConf.Srv.URL, "rhel/8/x86_64/edge"}: fmt.Sprintf("ostree repository \"%s/refs/heads/rhel/8/x86_64/edge\" returned status: 404 Not Found", srvConf.Srv.URL),
			{srvConf.Srv.URL, "test_forbidden"}:     fmt.Sprintf("ostree repository \"%s/refs/heads/test_forbidden\" returned status: 403 Forbidden", srvConf.Srv.URL),
			{srvConf.Srv.URL, "get_bad_ref"}:        fmt.Sprintf("ostree repository \"%s/refs/heads/get_bad_ref\" returned invalid reference", srvConf.Srv.URL),
		}
		for in, expMsg := range errCases {
			_, err := ResolveRef(in.location, in.ref, srvConf.RHSM, srvConf.Subs, &mTLSSrv.CAPath)
			assert.EqualError(t, err, expMsg)
		}
	}
}

func TestVerifyRef(t *testing.T) {
	cases := map[string]bool{
		"a_perfectly_valid_ref": true,
		"another/valid/ref":     true,
		"this-one-has/all.the/_valid-/characters/even/_numbers_42": true,
		"rhel/8/aarch64/edge": true,
		"1337":                true,
		"1337/but/also/more":  true,
		"_good_start/ref":     true,
		"/bad/ref":            false,
		"invalid)characters":  false,
		"this/was/doing/fine/until/the/very/end/": false,
		"-bad_start/ref":     false,
		".another/bad/start": false,
		"how/about/now?":     false,
	}

	for in, expOut := range cases {
		assert.Equal(t, expOut, VerifyRef(in), in)
	}
}
