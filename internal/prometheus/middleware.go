package prometheus

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/openshift-online/ocm-sdk-go/metrics"
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

func OcmPrometheusMiddleware(subsystem string, prefix string, paths []string) func(next echo.HandlerFunc) echo.HandlerFunc {
	builder := metrics.NewHandlerWrapper().Subsystem(subsystem)

	for _, path := range paths {
		// this will build the pathTree for the various endpoints
		builder.Path(fmt.Sprintf("%s%s", prefix, path))
	}

	// this function registers all the prometheus metrics
	// and we don't want to rebuild for each request
	metricsWrapper, err := builder.Build()
	if err != nil {
		// programming error
		panic(err)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {

			// this feels super hacky but needs to be set before`next(c)`
			// otherwise the content-type gets set to application/text
			c.Response().Header().Set("Content-Type", "application/json")

			// we need to get the error codes from the error handling
			// this is one approach to doing it see:
			// -  https://github.com/labstack/echo/discussions/1820#discussioncomment-529428
			// -  https://github.com/labstack/echo/issues/1837#issuecomment-816399630
			err = next(c)
			status := c.Response().Status
			httpErr := new(echo.HTTPError)
			if errors.As(err, &httpErr) {
				status = httpErr.Code
			}

			metricsWrapper.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == 0 {
					// handle a zero status code since this is
					// most likely an internal server error
					status = http.StatusInternalServerError
				}
				w.WriteHeader(status)
				c.SetRequest(r)
				c.SetResponse(echo.NewResponse(w, c.Echo()))
			})).ServeHTTP(c.Response(), c.Request())
			return
		}
	}
}
