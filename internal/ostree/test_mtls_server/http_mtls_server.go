package test_mtls_server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
)

type MTLSServer struct {
	Server        *httptest.Server
	CAPath        string
	ClientKeyPath string
	ClientCrtPath string
}

func NewMTLSServer(handler http.Handler) (*MTLSServer, error) {
	certsPath, err := filepath.Abs("test_mtls_server")
	if err != nil {
		return nil, err
	}

	caPath := filepath.Join(certsPath, "ca.crt")
	serverKeyPath := filepath.Join(certsPath, "server.key")
	serverCrtPath := filepath.Join(certsPath, "server.crt")
	clientKeyPath := filepath.Join(certsPath, "client.key")
	clientCrtPath := filepath.Join(certsPath, "client.crt")

	caCertPem, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM(caCertPem)
	if !ok {
		return nil, fmt.Errorf("Unable to append ca cert (%s) to ca pool", caPath)
	}

	serverCrt, err := tls.LoadX509KeyPair(serverCrtPath, serverKeyPath)
	if err != nil {
		return nil, err
	}

	server := httptest.NewUnstartedServer(handler)

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCrt},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, chain := range verifiedChains {
				for _, domain := range []string{"127.0.0.1", "localhost", "osbuild.org"} {
					if chain[0].VerifyHostname(domain) == nil {
						return nil
					}
				}
			}

			return errors.New("domain not in allowlist")
		},
	}
	server.StartTLS()

	return &MTLSServer{
		Server:        server,
		CAPath:        caPath,
		ClientKeyPath: clientKeyPath,
		ClientCrtPath: clientCrtPath,
	}, nil
}
