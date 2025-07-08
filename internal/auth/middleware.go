package auth

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v4"
)

const TenantCtxKey string = "tenant"

func TenantChannelMiddleware(tenantProviderFields []string, onFail error) func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			tenant, err := GetFromClaims(ctx.Request().Context(), tenantProviderFields)
			// Allowlisted paths won't have a token
			if err != nil && !errors.Is(err, ErrNoJWT) {
				return onFail
			}

			// prefix the tenant to prevent collisions if support for specifying channels in a request is ever added
			if tenant != "" {
				ctx.Set(TenantCtxKey, fmt.Sprintf("org-%s", tenant))
			}

			return next(ctx)
		}
	}
}
