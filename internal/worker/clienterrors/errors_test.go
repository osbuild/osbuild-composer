package clienterrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorBuildVersionMismatch(t *testing.T) {
	err := New(ErrorBuildVersionMismatch, "test reason", "test details")

	assert.Equal(t, ErrorBuildVersionMismatch, err.ID)
	assert.Equal(t, "test reason", err.Reason)
	assert.Equal(t, "test details", err.Details)

	// Should be a dependency error
	assert.True(t, err.IsDependencyError(), "ErrorBuildVersionMismatch should be a dependency error")

	// Should map to internal error status
	statusCode := GetStatusCode(err)
	assert.Equal(t, StatusCode(JobStatusInternalError), statusCode)
}
