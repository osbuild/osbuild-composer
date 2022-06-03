package v2

import (
	"github.com/labstack/echo/v4"
	"github.com/osbuild/osbuild-composer/internal/auth"
)

// getTenantChannel returns the tenant channel for the provided request context
func (s *Server) getTenantChannel(ctx echo.Context) (string, error) {
	// channel is empty if JWT is not enabled
	var channel string
	if s.config.JWTEnabled {
		tenant, err := auth.GetFromClaims(ctx.Request().Context(), s.config.TenantProviderFields)
		if err != nil {
			return "", err
		}
		// prefix the tenant to prevent collisions if support for specifying channels in a request is ever added
		channel = "org-" + tenant
	}
	return channel, nil
}
