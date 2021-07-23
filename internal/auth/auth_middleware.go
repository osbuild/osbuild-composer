package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type JWTMiddleware interface {
	AuthenticateAccountJWT(next http.Handler) http.Handler
}

type AuthMiddleware struct{}

var _ JWTMiddleware = &AuthMiddleware{}

func NewAuthMiddleware() (*AuthMiddleware, error) {
	middleware := AuthMiddleware{}
	return &middleware, nil
}

// Middleware handler to validate JWT tokens and authenticate users
func (a *AuthMiddleware) AuthenticateAccountJWT(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		payload, err := GetAuthPayloadFromContext(ctx.Request().Context())
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Errof("Unable to get payload details from JWT token: %s", err))
		}

		ctx.Set(UsernameKey, payload.Username)

		// TODO sentry support
		// // Add username to sentry context
		// if hub := sentry.GetHubFromContext(ctx); hub != nil {
		// 	hub.ConfigureScope(func(scope *sentry.Scope) {
		// 		scope.SetUser(sentry.User{ID: payload.Username})
		// 	})
		// }
		return nextHandler(eCtx)
	}
}
