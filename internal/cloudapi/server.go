package cloudapi

import (
	"net/http"

	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/reporegistry"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type Server struct {
	v2 *v2.Server
}

func NewServer(workers *worker.Server, distros *distrofactory.Factory, repos *reporegistry.RepoRegistry, solver *dnfjson.BaseSolver, config v2.ServerConfig) *Server {
	server := &Server{
		v2: v2.NewServer(workers, distros, repos, solver, config),
	}
	return server
}

func (server *Server) V2(path string) http.Handler {
	return server.v2.Handler(path)
}

func (server *Server) Shutdown() {
	server.v2.Shutdown()
}
