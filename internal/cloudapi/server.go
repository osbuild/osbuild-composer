package cloudapi

import (
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/worker"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

type Server struct {
	v2 *v2.Server
}

func NewServer(workers *worker.Server, distros *distroregistry.Registry, awsBucket string) *Server {
	server := &Server{
		v2: v2.NewServer(workers, distros, awsBucket),
	}
	return server
}

func (server *Server) V2(path string) http.Handler {
	return server.v2.Handler(path)
}
