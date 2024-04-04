package common

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/segmentio/ksuid"
)

type ctxKey string

const OperationIDKey string = "operationID"
const operationIDKeyCtx ctxKey = ctxKey(OperationIDKey)

// Adds a time-sortable globally unique identifier to an echo.Context if not already set
func OperationIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Get(OperationIDKey) == nil {
			oid := GenerateOperationID()
			c.Set(OperationIDKey, oid)

			ctx := c.Request().Context()
			ctx = context.WithValue(ctx, operationIDKeyCtx, oid)
			c.SetRequest(c.Request().WithContext(ctx))
		}

		return next(c)
	}
}

func GenerateOperationID() string {
	return ksuid.New().String()
}
