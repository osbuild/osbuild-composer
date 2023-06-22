package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func StatusRequestsCounter(subsystem string) *prometheus.CounterVec {
	// return a function so we can use this for both
	// composer & worker metrics
	reg := prometheus.NewRegistry()
	counter := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Name:      "request_count",
		Namespace: Namespace,
		Subsystem: subsystem,
		Help:      "total number of http requests",
	}, []string{"method", "path", "code", "subsystem", "tenant"})

	err := prometheus.Register(counter)
	if err != nil {
		registered, ok := err.(prometheus.AlreadyRegisteredError)
		if !ok {
			panic(err)
		}
		// return existing counter if metrics already registered
		return registered.ExistingCollector.(*prometheus.CounterVec)
	}

	return counter
}
