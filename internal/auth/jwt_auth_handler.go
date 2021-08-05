package auth

import (
	"context"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/openshift-online/ocm-sdk-go/authentication"
	"github.com/openshift-online/ocm-sdk-go/logging"
)

// When using this handler for auth, it should be run as high up as possible.
// Exceptions can be registered in the `exclude` slice
func BuildJWTAuthHandler(keysURL, caFile, aclFile string, exclude []string, next http.Handler) (handler http.Handler, err error) {
	logBuilder := logging.NewGoLoggerBuilder()
	if caFile != "" {
		logBuilder = logBuilder.Debug(true)
	}

	logger, err := logBuilder.Build()
	if err != nil {
		return
	}

	logger.Info(context.Background(), aclFile)

	builder := authentication.NewHandler().
		Logger(logger).
		KeysURL(keysURL)

	// Used during testing
	if caFile != "" {
		logger.Warn(context.Background(),
			"A custom CA is specified to verify jwt tokens, this shouldn't be enabled in a production setting.")
		caPEM, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM(caPEM)
		if !ok {
			return nil, fmt.Errorf("Unable to load jwt ca cert %s.", caFile)
		}
		builder = builder.KeysCAs(pool)
	}

	if aclFile != "" {
		builder = builder.ACLFile(aclFile)
	}

	for _, e := range exclude {
		builder = builder.Public(e)
	}
	handler, err = builder.Next(next).Build()
	return
}
