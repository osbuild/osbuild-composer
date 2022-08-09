//go:build integration
// +build integration

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const trustedCADir = "/etc/osbuild-composer-test/ca"

type connectionConfig struct {
	CACertFile     string
	ClientKeyFile  string
	ClientCertFile string
}

func createTLSConfig(config *connectionConfig) (*tls.Config, error) {
	caCertPEM, err := ioutil.ReadFile(config.CACertFile)
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertPEM)
	if !ok {
		return nil, errors.New("failed to append root certificate")
	}

	cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func TestWorkerAPIAuth(t *testing.T) {
	t.Run("certificate signed by a trusted CA", func(t *testing.T) {
		cases := []struct {
			caseDesc string
			subj     string
			addext   string
			success  bool
		}{
			{"valid CN 1", "/CN=worker.osbuild.org/emailAddress=osbuild@example.com", "subjectAltName=DNS:example.com,DNS:worker.osbuild.org", true},
			{"valid CN 2", "/CN=localhost/emailAddress=osbuild@example.com", "subjectAltName=DNS:example.com,DNS:localhost", true},
			{"invalid CN", "/CN=example.com/emailAddress=osbuild@example.com", "subjectAltName=DNS:example.com", false},
		}

		authority := &ca{BaseDir: trustedCADir}

		for _, c := range cases {
			t.Run(c.caseDesc, func(t *testing.T) {
				ckp, err := authority.newCertificateKeyPair(c.subj, osbuildClientExt, c.addext)
				require.NoError(t, err)
				defer ckp.remove()

				testRoute(t, "https://localhost:8700/api/worker/v1/status", ckp, c.success)
			})
		}
	})

	t.Run("certificate signed by an untrusted CA", func(t *testing.T) {
		// generate a new CA
		ca, err := newCA("/CN=untrusted.osbuild.org")
		require.NoError(t, err)
		defer ca.remove()

		// create a new certificate and signed it with the new CA
		ckp, err := ca.newCertificateKeyPair("/CN=localhost/emailAddress=osbuild@example.com", osbuildClientExt, "")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost:8700/api/worker/v1/status", ckp, false)
	})

	t.Run("self-signed certificate", func(t *testing.T) {
		// generate a new self-signed certificate
		ckp, err := newSelfSignedCertificateKeyPair("/CN=osbuild.org")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost:8700/api/worker/v1/status", ckp, false)
	})
}

func testRoute(t *testing.T, route string, ckp *certificateKeyPair, expectSuccess bool) {
	tlsConfig, err := createTLSConfig(&connectionConfig{
		CACertFile:     "/etc/osbuild-composer/ca-crt.pem",
		ClientKeyFile:  ckp.key(),
		ClientCertFile: ckp.certificate(),
	})
	require.NoError(t, err)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	client := http.Client{Transport: transport}

	response, err := client.Get(route)
	if expectSuccess {
		require.NoError(t, err)

		var status struct {
			Status string `json:"status"`
		}
		err := json.NewDecoder(response.Body).Decode(&status)
		require.NoError(t, err)

		require.Equal(t, "OK", status.Status)
	} else {
		require.Error(t, err)
	}
}
