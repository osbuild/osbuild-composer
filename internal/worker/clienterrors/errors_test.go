package clienterrors_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type customErr struct{}

func (ce *customErr) Error() string {
	return "customErr"
}

func TestErrorInterface(t *testing.T) {
	for _, tc := range []struct {
		err         error
		expectedStr string
	}{
		{fmt.Errorf("some error"), "some error"},
		{&customErr{}, "customErr"},
	} {
		wce := clienterrors.WorkerClientError(2, "details", tc.err)
		assert.Equal(t, fmt.Sprintf("Code: 2, Reason: details, Details: %s", tc.expectedStr), wce.String())
	}
}
