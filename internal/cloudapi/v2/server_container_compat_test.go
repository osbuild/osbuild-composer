package v2

import (
	"testing"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchContainerSpecsToPipelines(t *testing.T) {
	testCases := []struct {
		name              string
		resolved          []worker.ContainerSpec
		sourcesByPipeline map[string][]container.SourceSpec
		expected          map[string][]container.Spec
		wantErr           string
	}{
		{
			name: "single pipeline - all sources matched",
			resolved: []worker.ContainerSpec{
				{Source: "registry.example.com/image:latest", Name: "test", ImageID: "sha256:abc", Digest: "sha256:def"},
			},
			sourcesByPipeline: map[string][]container.SourceSpec{
				"image": {
					{Source: "registry.example.com/image:latest", Name: "test"},
				},
			},
			expected: map[string][]container.Spec{
				"image": {
					{Source: "registry.example.com/image:latest", LocalName: "test", ImageID: "sha256:abc", Digest: "sha256:def"},
				},
			},
		},
		{
			name: "multiple pipelines - distinct sources",
			resolved: []worker.ContainerSpec{
				{Source: "registry.example.com/buildroot:latest", Name: "buildroot", ImageID: "sha256:111", Digest: "sha256:222"},
				{Source: "registry.example.com/os:latest", Name: "os-content", ImageID: "sha256:333", Digest: "sha256:444"},
			},
			sourcesByPipeline: map[string][]container.SourceSpec{
				"build": {
					{Source: "registry.example.com/buildroot:latest", Name: "buildroot"},
				},
				"image": {
					{Source: "registry.example.com/os:latest", Name: "os-content"},
				},
			},
			expected: map[string][]container.Spec{
				"build": {
					{Source: "registry.example.com/buildroot:latest", LocalName: "buildroot", ImageID: "sha256:111", Digest: "sha256:222"},
				},
				"image": {
					{Source: "registry.example.com/os:latest", LocalName: "os-content", ImageID: "sha256:333", Digest: "sha256:444"},
				},
			},
		},
		{
			name: "multiple pipelines - shared source (bootc case)",
			resolved: []worker.ContainerSpec{
				{Source: "registry.example.com/bootc:latest", Name: "bootc-image", ImageID: "sha256:abc", Digest: "sha256:def"},
			},
			sourcesByPipeline: map[string][]container.SourceSpec{
				"build": {
					{Source: "registry.example.com/bootc:latest", Name: "bootc-image"},
				},
				"image": {
					{Source: "registry.example.com/bootc:latest", Name: "bootc-image"},
				},
			},
			expected: map[string][]container.Spec{
				"build": {
					{Source: "registry.example.com/bootc:latest", LocalName: "bootc-image", ImageID: "sha256:abc", Digest: "sha256:def"},
				},
				"image": {
					{Source: "registry.example.com/bootc:latest", LocalName: "bootc-image", ImageID: "sha256:abc", Digest: "sha256:def"},
				},
			},
		},
		{
			name: "missing source - error",
			resolved: []worker.ContainerSpec{
				{Source: "registry.example.com/other:latest", Name: "other", ImageID: "sha256:abc", Digest: "sha256:def"},
			},
			sourcesByPipeline: map[string][]container.SourceSpec{
				"image": {
					{Source: "registry.example.com/missing:latest", Name: "missing"},
				},
			},
			wantErr: `container source "registry.example.com/missing:latest" for pipeline "image" not found in resolved specs`,
		},
		{
			name:              "empty inputs",
			resolved:          nil,
			sourcesByPipeline: nil,
			expected:          map[string][]container.Spec{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := matchContainerSpecsToPipelines(tc.resolved, tc.sourcesByPipeline)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, tc.expected, result)
		})
	}
}
