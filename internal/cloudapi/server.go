package cloudapi

import (
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"

	v1 "github.com/osbuild/osbuild-composer/internal/cloudapi/v1"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

type Server struct {
	v1 *v1.Server
	v2 *v2.Server
}

func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distroregistry.Registry) *Server {
	server := &Server{
		v1: v1.NewServer(workers, rpmMetadata, distros),
		v2: v2.NewServer(workers, rpmMetadata, distros),
	}
	return server
}

func (server *Server) V1(path string) http.Handler {
	return server.v1.Handler(path)
}

func (server *Server) V2(path string) http.Handler {
	return server.v2.Handler(path)
}
