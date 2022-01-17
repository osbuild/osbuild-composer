package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const workerSubsystem = "composer_worker"

var (
	PendingJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "pending_jobs",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Currently pending jobs",
	}, []string{"type"})
)

var (
	RunningJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "running_jobs",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Currently running jobs",
	}, []string{"type"})
)

var (
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_duration_seconds",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Duration spent by workers on a job.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2049},
	}, []string{"type"})
)

var (
	JobWaitDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "job_wait_duration_seconds",
		Namespace: namespace,
		Subsystem: workerSubsystem,
		Help:      "Duration a job spends on the queue.",
		Buckets:   []float64{.1, .2, .5, 1, 2, 4, 8, 16, 32, 40, 48, 64, 96, 128, 160, 192, 224, 256, 320, 382, 448, 512, 640, 768, 896, 1024, 1280, 1536, 1792, 2049},
	}, []string{"type"})
)

func EnqueueJobMetrics(jobType string) {
	PendingJobs.WithLabelValues(jobType).Inc()
}

func DequeueJobMetrics(queued time.Time, started time.Time, jobType string) {
	if !started.IsZero() && !queued.IsZero() {
		diff := started.Sub(queued).Seconds()
		JobWaitDuration.WithLabelValues(jobType).Observe(diff)
		PendingJobs.WithLabelValues(jobType).Dec()
		RunningJobs.WithLabelValues(jobType).Inc()
	}
}

func CancelJobMetrics(started time.Time, jobType string) {
	if !started.IsZero() {
		RunningJobs.WithLabelValues(jobType).Dec()
	} else {
		PendingJobs.WithLabelValues(jobType).Dec()
	}
}

func FinishJobMetrics(started time.Time, finished time.Time, canceled bool, jobType string) {
	if !finished.IsZero() && !canceled {
		diff := finished.Sub(started).Seconds()
		JobDuration.WithLabelValues(jobType).Observe(diff)
		RunningJobs.WithLabelValues(jobType).Dec()
	}
}
