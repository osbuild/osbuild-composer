package remotefile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingleInputResolver(t *testing.T) {
	server := makeTestServer()
	url := server.URL + "/key1"

	resolver := NewResolver()

	expectedOutput := Spec{
		URL:             url,
		Content:         []byte("key1\n"),
		ResolutionError: nil,
	}

	resolver.Add(url)

	resultItems := resolver.Finish()
	assert.Contains(t, resultItems, expectedOutput)

	for _, item := range resultItems {
		assert.Nil(t, item.ResolutionError)
	}
}

func TestMultiInputResolver(t *testing.T) {
	server := makeTestServer()

	urlOne := server.URL + "/key1"
	urlTwo := server.URL + "/key2"

	expectedOutputOne := Spec{
		URL:             urlOne,
		Content:         []byte("key1\n"),
		ResolutionError: nil,
	}

	expectedOutputTwo := Spec{
		URL:             urlTwo,
		Content:         []byte("key2\n"),
		ResolutionError: nil,
	}

	resolver := NewResolver()

	resolver.Add(urlOne)
	resolver.Add(urlTwo)

	resultItems := resolver.Finish()

	assert.Contains(t, resultItems, expectedOutputOne)
	assert.Contains(t, resultItems, expectedOutputTwo)

	for _, item := range resultItems {
		assert.Nil(t, item.ResolutionError)
	}
}

func TestInvalidInputResolver(t *testing.T) {
	url := ""

	resolver := NewResolver()

	resolver.Add(url)

	expectedErr := fmt.Errorf("File resolver: url is required")

	resultItems := resolver.Finish()

	for _, item := range resultItems {
		assert.Equal(t, item.ResolutionError.Reason, expectedErr.Error())
	}
}

func TestMultiInvalidInputResolver(t *testing.T) {
	urlOne := ""
	urlTwo := "hello"

	resolver := NewResolver()

	resolver.Add(urlOne)
	resolver.Add(urlTwo)

	expectedErrMessageOne := "File resolver: url is required"
	expectedErrMessageTwo := fmt.Sprintf("File resolver: invalid url %s", urlTwo)

	resultItems := resolver.Finish()

	errs := []string{}
	for _, item := range resultItems {
		errs = append(errs, item.ResolutionError.Reason)
	}

	assert.Contains(t, errs, expectedErrMessageOne)
	assert.Contains(t, errs, expectedErrMessageTwo)
}
