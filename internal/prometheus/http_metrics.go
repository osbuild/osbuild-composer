package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TotalRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "total_requests",
		Namespace: Namespace,
		Subsystem: ComposerSubsystem,
		Help:      "total number of http requests made to osbuild-composer",
	})
)

var (
	ComposeRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name:      "total_compose_requests",
		Namespace: Namespace,
		Subsystem: ComposerSubsystem,
		Help:      "total number of compose requests made to osbuild-composer",
	})
)

func HTTPDurationHisto(subsystem string) *prometheus.HistogramVec {
	reg := prometheus.NewRegistry()
	histo := promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
		Name:      "http_duration_seconds",
		Namespace: Namespace,
		Subsystem: subsystem,
		Help:      "Duration of HTTP requests.",
		Buckets:   []float64{.025, .05, .075, .1, .2, .5, .75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10, 12, 14, 16, 20},
	}, []string{"path", "tenant"})

	err := prometheus.Register(histo)
	if err != nil {
		registered, ok := err.(prometheus.AlreadyRegisteredError)
		if !ok {
			panic(err)
		}
		// return existing counter if metrics already registered
		return registered.ExistingCollector.(*prometheus.HistogramVec)
	}
	return histo
}
