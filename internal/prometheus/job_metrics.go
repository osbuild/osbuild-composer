package prometheus

import (
	"time"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const workerSubsystem = "worker"

var (
	TotalJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "total_jobs",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Total jobs",
	}, []string{"type", "status", "tenant", "arch"})
)

var (
	PendingJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "pending_jobs",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Currently pending jobs",
	}, []string{"type", "tenant"})
)

var (
	RunningJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "running_jobs",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Currently running jobs",
	}, []string{"type", "tenant"})
)

var (
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_duration_seconds",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Duration spent by workers on a job.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2049},
	}, []string{"type", "status", "tenant", "arch"})
)

var (
	JobWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_wait_duration_seconds",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Duration a job spends on the queue.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2049},
	}, []string{"type", "tenant"})
)

func EnqueueJobMetrics(jobType, tenant string) {
	PendingJobs.WithLabelValues(jobType, tenant).Inc()
}

func DequeueJobMetrics(pending time.Time, started time.Time, jobType, tenant string) {
	if !started.IsZero() && !pending.IsZero() {
		diff := started.Sub(pending).Seconds()
		JobWaitDuration.WithLabelValues(jobType, tenant).Observe(diff)
		PendingJobs.WithLabelValues(jobType, tenant).Dec()
		RunningJobs.WithLabelValues(jobType, tenant).Inc()
	}
}

func CancelJobMetrics(started time.Time, jobType, tenant string) {
	if !started.IsZero() {
		RunningJobs.WithLabelValues(jobType, tenant).Dec()
	} else {
		PendingJobs.WithLabelValues(jobType, tenant).Dec()
	}
}

func FinishJobMetrics(started time.Time, finished time.Time, canceled bool, jobType, tenant, arch string, status clienterrors.StatusCode) {
	if !finished.IsZero() && !canceled {
		diff := finished.Sub(started).Seconds()
		JobDuration.WithLabelValues(jobType, status.ToString(), tenant, arch).Observe(diff)
		TotalJobs.WithLabelValues(jobType, status.ToString(), tenant, arch).Inc()
		RunningJobs.WithLabelValues(jobType, tenant).Dec()
	}
}
