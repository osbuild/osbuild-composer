package common

import (
	"github.com/labstack/echo/v4"
	"github.com/segmentio/ksuid"
)

const OperationIDKey string = "operationID"

// Adds a time-sortable globally unique identifier to an echo.Context if not already set
func OperationIDMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Get(OperationIDKey) == nil {
			c.Set(OperationIDKey, GenerateOperationID())
		}
		return next(c)
	}
}

func GenerateOperationID() string {
	return ksuid.New().String()
}
