package v2

import (
	"context"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// OverrideSerializeManifestFunc overrides the serializeManifestFunc for testing
func OverrideSerializeManifestFunc(f func(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, depsolveJobID, containerResolveJobID, ostreeResolveJobID, manifestJobID uuid.UUID, seed int64)) func() {
	originalSerializeManifestFunc := serializeManifestFunc
	serializeManifestFunc = f
	return func() {
		serializeManifestFunc = originalSerializeManifestFunc
	}
}
