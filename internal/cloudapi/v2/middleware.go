package v2

import (
	"github.com/getkin/kin-openapi/openapi3filter"
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

func (s *Server) ValidateRequest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		request := c.Request()

		// extract route and parameters from request
		route, params, err := s.router.FindRoute(request)
		if err != nil {
			return HTTPErrorWithInternal(ErrorResourceNotFound, err)
		}

		input := &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: params,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		}

		ctx := request.Context()
		if err := openapi3filter.ValidateRequest(ctx, input); err != nil {
			return HTTPErrorWithInternal(ErrorInvalidRequest, err)
		}

		return next(c)
	}
}
