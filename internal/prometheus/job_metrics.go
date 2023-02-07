package prometheus

import (
	"time"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TotalJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "total_jobs",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Total jobs",
	}, []string{"type", "status", "tenant", "arch"})
)

var (
	PendingJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "pending_jobs",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Currently pending jobs",
	}, []string{"type", "tenant"})
)

var (
	RunningJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "running_jobs",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Currently running jobs",
	}, []string{"type", "tenant"})
)

var (
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_duration_seconds",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Duration spent by workers on a job.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2048, 2304, 2560, 2816, 3072, 3328, 3584, 3840, 4096, 4608, 5120, 5632, 6144, 6656, 7168, 7680, 8192, 8704, 9216, 9728, 10240, 10752},
	}, []string{"type", "status", "tenant", "arch"})
)

var (
	JobWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_wait_duration_seconds",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Duration a job spends on the queue.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2048, 2304, 2560, 2816, 3072, 3328, 3584, 3840, 4096, 4608, 5120, 5632, 6144, 6656, 7168, 7680, 8192, 8704, 9216, 9728, 10240, 10752},
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
