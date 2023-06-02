package mock_ostree_repo

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
)

type OSTreeTestRepo struct {
	OSTreeRef string
	Server    *httptest.Server
}

func (repo *OSTreeTestRepo) TearDown() {
	if repo == nil {
		return
	}
	repo.Server.Close()
}

func Setup(ref string) *OSTreeTestRepo {
	repo := new(OSTreeTestRepo)
	repo.OSTreeRef = ref

	mux := http.NewServeMux()
	repo.Server = httptest.NewServer(mux)

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(repo.Server.URL+ref)))
	fmt.Printf("Creating repo with %s %s %s\n", ref, repo.Server.URL, checksum)
	mux.HandleFunc("/refs/heads/"+ref, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksum)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// catch-all handler, return 404
		http.NotFound(w, r)
	})

	return repo
}
