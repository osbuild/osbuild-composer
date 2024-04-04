package common

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
)

const ExternalIDKey string = "externalID"
const externalIDKeyCtx ctxKey = ctxKey(ExternalIDKey)

// Extracts HTTP header X-External-Id and sets it as a request context value
func ExternalIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		eid := c.Request().Header.Get("X-External-Id")
		if eid == "" {
			return next(c)
		}

		eid = strings.TrimSuffix(eid, "\n")
		c.Set(ExternalIDKey, eid)

		ctx := c.Request().Context()
		ctx = context.WithValue(ctx, externalIDKeyCtx, eid)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}
