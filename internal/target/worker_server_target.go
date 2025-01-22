package target

const TargetNameWorkerServer TargetName = "org.osbuild.worker.server"

type WorkerServerTargetOptions struct{}

func (WorkerServerTargetOptions) isTargetOptions() {}

func NewWorkerServerTarget() *Target {
	return newTarget(TargetNameWorkerServer, &WorkerServerTargetOptions{})
}

type WorkerServerTargetResultOptions struct {
	ArtifactRelPath string `json:"artifact_relative_path"`
}

func (WorkerServerTargetResultOptions) isTargetResultOptions() {}

func NewWorkerServerTargetResult(options *WorkerServerTargetResultOptions, artifact *OsbuildArtifact) *TargetResult {
	return newTargetResult(TargetNameWorkerServer, options, artifact)
}
