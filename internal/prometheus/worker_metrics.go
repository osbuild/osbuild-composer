package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ActiveWorkers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "active_workers",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Active workers",
	}, []string{"worker", "tenant", "arch"})
)

func AddActiveWorker(worker, tenant, arch string) {
	ActiveWorkers.WithLabelValues(worker, tenant, arch).Inc()
}

func RemoveActiveWorker(worker, tenant, arch string) {
	ActiveWorkers.WithLabelValues(worker, tenant, arch).Dec()
}
