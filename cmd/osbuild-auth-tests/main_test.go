// +build integration

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	}, nil
}

type certificateKeyPair struct {
	baseDir string
}

func (ckp certificateKeyPair) remove() {
	err := os.RemoveAll(ckp.baseDir)
	if err != nil {
		log.Printf("cannot delete the certificate key pair: %v", err)
	}
}

func (ckp certificateKeyPair) certificate() string {
	return path.Join(ckp.baseDir, "crt")
}

func (ckp certificateKeyPair) key() string {
	return path.Join(ckp.baseDir, "key")
}

func newCertificateKeyPair(CA, CAkey, subj string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}
	certificateRequest := path.Join(dir, "csr")

	cmd := exec.Command(
		"openssl", "req", "-new", "-nodes",
		"-subj", subj,
		"-keyout", ckp.key(),
		"-out", certificateRequest,
	)

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a private key and a certificate request: %v", err)
	}

	defer os.Remove(certificateRequest)

	cmd = exec.Command(
		"openssl", "x509", "-req", "-CAcreateserial",
		"-in", certificateRequest,
		"-CA", CA,
		"-CAkey", CAkey,
		"-out", ckp.certificate(),
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot sign the certificate: %v", err)
	}

	return &ckp, nil
}

func newSelfSignedCertificateKeyPair(subj string) (*certificateKeyPair, error) {
	dir, err := ioutil.TempDir("", "osbuild-auth-tests-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a temporary directory for the certificate: %v", err)
	}

	ckp := certificateKeyPair{baseDir: dir}

	cmd := exec.Command(
		"openssl", "req", "-nodes", "-x509",
		"-subj", subj,
		"-out", ckp.certificate(),
		"-keyout", ckp.key(),
	)
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot generate a self-signed certificate: %v", err)
	}

	return &ckp, nil
}

func TestWorkerAPIAuth(t *testing.T) {
	t.Run("certificate signed by a trusted CA", func(t *testing.T) {
		cases := []struct {
			caseDesc string
			subj     string
			success  bool
		}{
			{"valid CN 1", "/CN=worker.osbuild.org", true},
			{"valid CN 2", "/CN=localhost", true},
			{"invalid CN", "/CN=example.com", false},
		}

		for _, c := range cases {
			t.Run(c.caseDesc, func(t *testing.T) {
				ckp, err := newCertificateKeyPair("/etc/osbuild-composer/ca-crt.pem", "/etc/osbuild-composer/ca-key.pem", c.subj)
				require.NoError(t, err)
				defer ckp.remove()

				testRoute(t, "https://localhost:8700/status", ckp, c.success)
			})
		}
	})

	t.Run("certificate signed by an untrusted CA", func(t *testing.T) {
		// generate a new CA
		ca, err := newSelfSignedCertificateKeyPair("/CN=osbuild.org")
		require.NoError(t, err)
		defer ca.remove()

		// create a new certificate and signed it with the new CA
		ckp, err := newCertificateKeyPair(ca.certificate(), ca.key(), "/CN=localhost")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost:8700/status", ckp, false)
	})

	t.Run("self-signed certificate", func(t *testing.T) {
		// generate a new self-signed certificate
		ckp, err := newSelfSignedCertificateKeyPair("/CN=osbuild.org")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost:8700/status", ckp, false)
	})
}

func TestKojiAPIAuth(t *testing.T) {
	t.Run("certificate signed by a trusted CA", func(t *testing.T) {
		cases := []struct {
			caseDesc string
			subj     string
			success  bool
		}{
			{"valid CN 1", "/CN=worker.osbuild.org", true},
			{"valid CN 2", "/CN=localhost", true},
			{"invalid CN", "/CN=example.com", false},
		}

		for _, c := range cases {
			t.Run(c.caseDesc, func(t *testing.T) {
				ckp, err := newCertificateKeyPair("/etc/osbuild-composer/ca-crt.pem", "/etc/osbuild-composer/ca-key.pem", c.subj)
				require.NoError(t, err)
				defer ckp.remove()

				testRoute(t, "https://localhost/status", ckp, c.success)
			})
		}
	})

	t.Run("certificate signed by an untrusted CA", func(t *testing.T) {
		// generate a new CA
		ca, err := newSelfSignedCertificateKeyPair("/CN=osbuild.org")
		require.NoError(t, err)
		defer ca.remove()

		// create a new certificate and signed it with the new CA
		ckp, err := newCertificateKeyPair(ca.certificate(), ca.key(), "/CN=localhost")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost/status", ckp, false)
	})

	t.Run("self-signed certificate", func(t *testing.T) {
		// generate a new self-signed certificate
		ckp, err := newSelfSignedCertificateKeyPair("/CN=osbuild.org")
		require.NoError(t, err)
		defer ckp.remove()

		testRoute(t, "https://localhost/status", ckp, false)
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
