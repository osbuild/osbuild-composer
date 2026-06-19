package cert

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

var ErrNoValidPEMCertificatesFound = errors.New("no valid PEM certificates found")

// ParseCerts parses a PEM-encoded certificate chain formatted as concatenated strings
// and returns a slice of x509.Certificate. In case of unparsable certificates, the
// function returns an empty slice.
// Returns an error when a cert cannot be parsed, or when no certificates are recognized
// in the input.
func ParseCerts(cert string) ([]*x509.Certificate, error) {
	result := make([]*x509.Certificate, 0, 1)
	block := []byte(cert)
	var blocks [][]byte
	for {
		var certDERBlock *pem.Block
		certDERBlock, block = pem.Decode(block)
		if certDERBlock == nil {
			break
		}

		if certDERBlock.Type == "CERTIFICATE" {
			blocks = append(blocks, certDERBlock.Bytes)
		}
	}

	for _, block := range blocks {
		cert, err := x509.ParseCertificate(block)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		result = append(result, cert)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%w in: %s", ErrNoValidPEMCertificatesFound, cert)
	}

	return result, nil
}
