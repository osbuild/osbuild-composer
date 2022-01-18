package mock_ostree_repo

import (
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
	mux.HandleFunc("/refs/heads/"+ref, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "02604b2da6e954bd34b8b82a835e5a77d2b60ffa")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// catch-all handler, return 404
		http.NotFound(w, r)
	})

	repo.Server = httptest.NewServer(mux)

	return repo
}
