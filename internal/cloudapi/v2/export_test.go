package v2

import (
	"context"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type ManifestJobDependencies = manifestJobDependencies

// MockSerializeManifestFunc overrides the serializeManifestFunc for testing
func MockSerializeManifestFunc(f func(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, dependencies manifestJobDependencies, manifestJobID uuid.UUID, seed int64)) (restore func()) {
	originalSerializeManifestFunc := serializeManifestFunc
	serializeManifestFunc = f
	return func() {
		serializeManifestFunc = originalSerializeManifestFunc
	}
}
