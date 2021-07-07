package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TotalRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_http_requests",
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
