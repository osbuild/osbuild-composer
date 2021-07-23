package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type JWTMiddleware interface {
	ValidateJWT(next echo.HandlerFunc) echo.HandlerFunc
}

type AuthMiddleware struct {
}

var _ JWTMiddleware = &AuthMiddleware{}

// Middleware handler to validate JWT tokens and authenticate users // TODO this just parses the ctx
func (a *AuthMiddleware) ValidateJWT(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		payload, err := GetAuthPayloadFromContext(ctx.Request().Context())
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("Unable to get payload details from JWT token: %s", err))
		}

		ctx.Set(UsernameKey, payload.Username)
		return next(ctx)
	}
}
