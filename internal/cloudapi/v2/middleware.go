package v2

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/auth"
)

const TenantCtxKey string = "tenant"

// getTenantChannel returns the tenant channel for the provided request context
func (s *Server) getTenantChannel(ctx echo.Context) (string, error) {
	if s.config.JWTEnabled {
		tenant, ok := ctx.Get(auth.TenantCtxKey).(string)
		if !ok {
			return "", HTTPError(ErrorTenantNotInContext)
		}
		return tenant, nil
	}
	// channel is empty if JWT is not enabled
	return "", nil
}

type ComposeHandlerFunc func(ctx echo.Context, jobId uuid.UUID) error

// Ensures that the job's channel matches the JWT cannel set in the echo.Context
func (s *Server) EnsureJobChannel(next ComposeHandlerFunc) ComposeHandlerFunc {
	return func(c echo.Context, jobId uuid.UUID) error {
		ctxChannel, err := s.getTenantChannel(c)
		if err != nil {
			return err
		}

		jobChannel, err := s.workers.JobChannel(jobId)
		if err != nil {
			fmt.Println(err)
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		if jobChannel != ctxChannel {
			return HTTPError(ErrorComposeNotFound)
		}

		return next(c, jobId)
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
			details := ""
			re, ok := err.(*openapi3filter.RequestError)
			if ok {
				details = re.Error()
			}
			return HTTPErrorWithDetails(ErrorValidationFailed, err, details)
		}

		return next(c)
	}
}
