package v2

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type ManifestJobDependencies = manifestJobDependencies

// ManifestSourceFunc exports the manifestSourceFunc type for testing.
type ManifestSourceFunc = manifestSourceFunc

// MockSerializeManifestFunc overrides the serializeManifestFunc for testing
func MockSerializeManifestFunc(f func(ctx context.Context, getManifestSource manifestSourceFunc, workers *worker.Server, dependencies manifestJobDependencies, manifestJobID uuid.UUID, seed int64)) (restore func()) {
	originalSerializeManifestFunc := serializeManifestFunc
	serializeManifestFunc = f
	return func() {
		serializeManifestFunc = originalSerializeManifestFunc
	}
}

// HandleBootcPreManifest exports the handleBootcPreManifest function for testing.
func HandleBootcPreManifest(workers *worker.Server, jobID uuid.UUID, token uuid.UUID, staticArgs json.RawMessage, dynArgs []json.RawMessage) {
	handleBootcPreManifest(workers, jobID, token, staticArgs, dynArgs)
}

// SerializeManifest exports the serializeManifest function for testing.
func SerializeManifest(ctx context.Context, getManifestSource ManifestSourceFunc, workers *worker.Server, dependencies ManifestJobDependencies, manifestJobID uuid.UUID, seed int64) {
	serializeManifest(ctx, getManifestSource, workers, dependencies, manifestJobID, seed)
}

// NewManifestJobDependencies creates a manifestJobDependencies for testing.
func NewManifestJobDependencies(
	depsolveJobID, containerResolveJobID, ostreeResolveJobID,
	bootcInfoResolveJobID, bootcPreManifestJobID uuid.UUID,
) ManifestJobDependencies {
	return manifestJobDependencies{
		depsolveJobID:         depsolveJobID,
		containerResolveJobID: containerResolveJobID,
		ostreeResolveJobID:    ostreeResolveJobID,
		bootcInfoResolveJobID: bootcInfoResolveJobID,
		bootcPreManifestJobID: bootcPreManifestJobID,
	}
}
