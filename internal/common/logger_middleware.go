package common

import (
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

// Store context in request logger to propagate correlation ids
func LoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ell := NewEchoLogrusLogger(logrus.StandardLogger(), c.Request().Context())
		defer ell.Close()

		c.SetLogger(ell)
		return next(c)
	}
}
