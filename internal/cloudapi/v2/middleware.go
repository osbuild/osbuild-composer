package v2

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
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

type ComposeHandlerFunc func(ctx echo.Context, id string) error

// Ensures that the job's channel matches the JWT cannel set in the echo.Context
func (s *Server) EnsureJobChannel(next ComposeHandlerFunc) ComposeHandlerFunc {
	return func(c echo.Context, id string) error {
		jobId, err := uuid.Parse(id)
		if err != nil {
			return HTTPError(ErrorInvalidComposeId)
		}

		ctxChannel, err := s.getTenantChannel(c)
		if err != nil {
			return HTTPErrorWithInternal(ErrorTenantNotFound, err)
		}

		jobChannel, err := s.workers.JobChannel(jobId)
		if err != nil {
			fmt.Println(err)
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		if jobChannel != ctxChannel {
			return HTTPError(ErrorComposeNotFound)
		}

		return next(c, id)
	}
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
