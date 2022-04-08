package rpmrepo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type testRepoServer struct {
	Server     *httptest.Server
	RepoConfig rpmmd.RepoConfig
}

func NewTestServer() *testRepoServer {
	server := httptest.NewServer(http.FileServer(http.Dir("../../test/data/testrepo/")))
	testrepo := rpmmd.RepoConfig{
		Name:      "cs9-baseos",
		BaseURL:   server.URL,
		CheckGPG:  false,
		IgnoreSSL: true,
		RHSM:      false,
	}
	return &testRepoServer{Server: server, RepoConfig: testrepo}
}

func (trs *testRepoServer) Close() {
	trs.Server.Close()
}

// WriteConfig writes the repository config to the file defined by the given
// path. Assumes the location already exists.
func (trs *testRepoServer) WriteConfig(path string) {
	fp, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	data, err := json.Marshal(trs.RepoConfig)
	if err != nil {
		panic(err)
	}
	if _, err := fp.Write(data); err != nil {
		panic(err)
	}
}
