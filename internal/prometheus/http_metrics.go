package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TotalRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "total_requests",
		Subsystem: ComposerSubsystem,
		Help:      "total number of http requests made to osbuild-composer",
	})
)

var (
	// update this to count all 500s
	ComposeFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "total_failed_compose_requests",
		Subsystem: ComposerSubsystem,
		Help:      "total number of failed compose requests",
	})
)

var (
	ComposeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "total_compose_requests",
		Subsystem: ComposerSubsystem,
		Help:      "total number of compose requests made to osbuild-composer",
	})
)

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "http_duration_seconds",
		Subsystem: ComposerSubsystem,
		Help:      "Duration of HTTP requests.",
		Buckets:   []float64{.025, .05, .075, .1, .2, .5, .75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 14, 16, 20},
	}, []string{"path"})
)
