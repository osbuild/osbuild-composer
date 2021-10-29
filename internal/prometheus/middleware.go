package prometheus

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

func MetricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		TotalRequests.Inc()
		if strings.HasSuffix(ctx.Path(), "/compose") {
			ComposeRequests.Inc()
		}
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(ctx.Path()))
		defer timer.ObserveDuration()
		return next(ctx)
	}
}
