package main_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrivialRootEndpoint(t *testing.T) {
	baseURL, _, loggerHook := runTestServer(t)

	endpoint := baseURL
	resp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	assert.Equal(t, loggerHook.LastEntry().Message, "/ handler called")
}
