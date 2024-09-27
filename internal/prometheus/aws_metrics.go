package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	UploadedS3Files = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "uploaded_s3_files",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Total files uploaded to S3",
	}, []string{"result"})

	RegisterImportSnapshot = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "register_import_snapshot_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of import snapshot registration",
	})

	CopyImage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "copy_image_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of copying image",
	})

	ShareImage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "share_image_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of sharing image",
	})

	RemoveImage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "remove_image_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of removing image",
	})

	RunSecureImage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "run_secure_image_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of staring secure image",
	})

	TerminateSecureImage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "terminate_secure_image_duration_seconds",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Duration of terminating secure image",
	})

	CreatedFleets = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "created_fleets",
		Namespace: Namespace,
		Subsystem: AWSSubsystem,
		Help:      "Number of created fleets",
	}, []string{"result", "type"})
)

func ObserveRegisterImportSnapshot() ObserveFunc {
	pt := prometheus.NewTimer(RegisterImportSnapshot)
	return pt.ObserveDuration
}

func ObserverCopyImage() ObserveFunc {
	pt := prometheus.NewTimer(CopyImage)
	return pt.ObserveDuration
}

func ShareImageObserver() ObserveFunc {
	pt := prometheus.NewTimer(ShareImage)
	return pt.ObserveDuration
}

func RemoveImageObserver() ObserveFunc {
	pt := prometheus.NewTimer(RemoveImage)
	return pt.ObserveDuration
}

func RunSecureImageObserver() ObserveFunc {
	pt := prometheus.NewTimer(RunSecureImage)
	return pt.ObserveDuration
}

func TerminateSecureImageObserver() ObserveFunc {
	pt := prometheus.NewTimer(TerminateSecureImage)
	return pt.ObserveDuration
}
