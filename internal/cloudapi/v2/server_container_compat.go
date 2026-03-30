package v2

import (
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// matchContainerSpecsToPipelines reconstructs a pipeline-keyed container spec
// map from flat resolved specs by matching on the Source field.
//
// This is a backward compatibility fallback, exercised only when an old worker
// returns results without PipelineSpecs. It can be removed once all workers
// are updated to produce pipeline-aware results.
func matchContainerSpecsToPipelines(
	resolved []worker.ContainerSpec,
	sourcesByPipeline map[string][]container.SourceSpec,
) (map[string][]container.Spec, error) {
	resolvedBySource := make(map[string]container.Spec, len(resolved))
	for _, s := range resolved {
		// NOTE: this silently drops any duplicate sources and uses the last one.
		// This is acceptable, because the assumption is that the same source
		// would be resolved to the same spec.
		resolvedBySource[s.Source] = s.ToVendorSpec()
	}

	result := make(map[string][]container.Spec, len(sourcesByPipeline))
	for pipeline, sources := range sourcesByPipeline {
		specs := make([]container.Spec, len(sources))
		for i, src := range sources {
			resolved, ok := resolvedBySource[src.Source]
			if !ok {
				return nil, fmt.Errorf(
					"container source %q for pipeline %q not found in resolved specs (old worker fallback)",
					src.Source, pipeline)
			}
			specs[i] = resolved
		}
		result[pipeline] = specs
	}
	return result, nil
}
