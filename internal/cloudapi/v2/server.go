package v2

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the cloud Server
type Server struct {
	workers *worker.Server
	distros *distroregistry.Registry
	config  ServerConfig
}

type ServerConfig struct {
	AWSBucket            string
	TenantProviderFields []string
	JWTEnabled           bool
}

func NewServer(workers *worker.Server, distros *distroregistry.Registry, config ServerConfig) *Server {
	server := &Server{
		workers: workers,
		distros: distros,
		config:  config,
	}
	return server
}

func (s *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.HTTPErrorHandler = s.HTTPErrorHandler
	e.Pre(common.OperationIDMiddleware)
	e.Use(middleware.Recover())
	e.Logger = common.Logger()

	handler := apiHandlers{
		server: s,
	}
	RegisterHandlers(e.Group(path, prometheus.MetricsMiddleware), &handler)

	return e
}
