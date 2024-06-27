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
		Buckets:   []float64{.1, .2, .5, 1, 2.5, 5, 10, 20, 30, 40, 60, 90, 120, 150, 180, 240, 300, 360, 420, 480, 540, 600, 720, 840, 960, 1080, 1200, 1320, 1440, 1560, 1680, 1800, 2100, 2400, 2700, 3000, 3600, 4800, 6000, 7200, 9000, 10800, 12600, 14400, 16200, 18000, 19800, 24000, 27000, 30000, 33000, 36000, 39600, 43200, 57600, 86400},
	}, []string{"type", "status", "tenant", "arch"})
)

var (
	JobWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_wait_duration_seconds",
		Namespace: Namespace,
		Subsystem: WorkerSubsystem,
		Help:      "Duration a job spends on the queue.",
		Buckets:   []float64{.1, .2, .5, 1, 2.5, 5, 10, 20, 30, 40, 60, 90, 120, 150, 180, 240, 300, 360, 420, 480, 540, 600, 720, 840, 960, 1080, 1200, 1320, 1440, 1560, 1680, 1800, 2100, 2400, 2700, 3000, 3600, 4800, 6000, 7200, 9000, 10800, 12600, 14400, 16200, 18000, 19800, 24000, 27000, 30000, 33000, 36000, 39600, 43200, 57600, 86400},
	}, []string{"type", "tenant", "arch"})
)

func EnqueueJobMetrics(jobType, tenant string) {
	PendingJobs.WithLabelValues(jobType, tenant).Inc()
}

func DequeueJobMetrics(pending time.Time, started time.Time, jobType, tenant, arch string) {
	if !started.IsZero() && !pending.IsZero() {
		diff := started.Sub(pending).Seconds()
		JobWaitDuration.WithLabelValues(jobType, tenant, arch).Observe(diff)
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
