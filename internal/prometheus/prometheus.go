package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TotalRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "composer_total_http_requests",
		Help: "total number of http requests made to osbuild-composer",
	})
)

var (
	ComposeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_compose_requests",
		Help: "total number of compose requests made to osbuild-composer",
	})
)

var (
	ComposeSuccesses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_successful_compose_requests",
		Help: "total number of successful compose requests",
	})
)

var (
	ComposeFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_failed_compose_requests",
		Help: "total number of failed compose requests",
	})
)
