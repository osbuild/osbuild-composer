package remotefile

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestServer() *httptest.Server {
	// use a simple mock server to test the client
	// and file content resolver
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key1" {
			fmt.Fprintln(w, "key1")
		}
		if r.URL.Path == "/key2" {
			fmt.Fprintln(w, "key2")
		}
	}))
}

func TestClientResolve(t *testing.T) {
	server := makeTestServer()

	url := server.URL + "/key1"

	client := NewClient()

	output, err := client.Resolve(url)
	assert.NoError(t, err)

	expectedOutput := "key1\n"

	assert.Equal(t, expectedOutput, string(output))
}

func TestInputSpecValidation(t *testing.T) {
	server := makeTestServer()

	test := []struct {
		name string
		url  string
		want error
	}{
		{
			name: "valid input spec",
			url:  server.URL + "/key1",
			want: nil,
		},
		{
			name: "missing url spec",
			url:  "",
			want: fmt.Errorf("File resolver: url is required"),
		},
	}

	client := NewClient()

	for _, tt := range test {
		url, err := client.validateURL(tt.url)
		if tt.want == nil {
			assert.NoError(t, err)
			assert.Equal(t, tt.url, url.String())
		} else {
			assert.EqualError(t, err, tt.want.Error())
			assert.Nil(t, url)
		}
	}

}
