package main_test

import (
	"testing"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/stretchr/testify/assert"
)

func TestParseManifestPipelines(t *testing.T) {
	type testCase struct {
		Manifest []byte

		ExpectedPipelines *worker.PipelineNames
		ExpectedError     string
	}

	testCases := map[string]testCase{
		"empty": {
			ExpectedError: "manifest is empty",
		},
		"bad-json": {
			Manifest:      []byte(`{not json}`),
			ExpectedError: "invalid character 'n' looking for beginning of object key string",
		},
		"bad-manifest": {
			Manifest:      []byte(`{"unknown-key": "value"}`),
			ExpectedError: `unexpected manifest version:  != 2`,
		},
		"bad-version": {
			Manifest:      []byte(`{"version": "42"}`),
			ExpectedError: `unexpected manifest version: 42 != 2`,
		},
		"no-pipelines": {
			Manifest:      []byte(`{"version": "2"}`),
			ExpectedError: `no pipelines found`,
		},
		"bad-build": {
			Manifest: []byte(`{
	"version": "2",
	"pipelines": [
		{
			"name": "build-pipeline"
		},
		{
			"name": "root-tree",
			"build": "build-pipeline"
		}
	],
	"sources": []
}`),
			ExpectedError: `unexpected pipeline build property format: build-pipeline`,
		},

		"simple": {
			Manifest: []byte(`{
	"version": "2",
	"pipelines": [
		{
			"name": "pipeline"
		}
	],
	"sources": []
}`),
			ExpectedPipelines: &worker.PipelineNames{
				Payload: []string{"pipeline"},
			},
		},

		"with-build": {
			Manifest: []byte(`{
	"version": "2",
	"pipelines": [
		{
			"name": "build-pipeline"
		},
		{
			"name": "root-tree",
			"build": "name:build-pipeline"
		},
		{
			"name": "image",
			"build": "name:build-pipeline"
		}
	],
	"sources": []
}`),
			ExpectedPipelines: &worker.PipelineNames{
				Build:   []string{"build-pipeline"},
				Payload: []string{"root-tree", "image"},
			},
		},

		"real-simplified": { // Real manifest, but simplified by removing stage options, inputs, and sources
			Manifest: []byte(`{
  "version": "2",
  "pipelines": [
    {
      "name": "build",
      "runner": "org.osbuild.fedora42",
      "stages": [
        {
          "type": "org.osbuild.rpm",
          "inputs": {},
          "options": {}
        },
        {
          "type": "org.osbuild.selinux",
          "options": {}
        }
      ]
    },
    {
      "name": "os",
      "build": "name:build",
      "stages": [
        {
          "type": "org.osbuild.rpm",
          "inputs": {},
          "options": {}
        },
        {
          "type": "org.osbuild.fix-bls",
          "options": {}
        },
        {
          "type": "org.osbuild.locale",
          "options": {
            "language": "C.UTF-8"
          }
        },
        {
          "type": "org.osbuild.hostname",
          "options": {}
        },
        {
          "type": "org.osbuild.timezone",
          "options": {}
        },
        {
          "type": "org.osbuild.machine-id",
          "options": {}
        }
      ]
    },
    {
      "name": "archive",
      "build": "name:build",
      "stages": [
        {
          "type": "org.osbuild.tar",
          "inputs": {},
          "options": {}
        }
      ]
    },
    {
      "name": "xz",
      "build": "name:build",
      "stages": [
        {
          "type": "org.osbuild.xz",
          "inputs": {},
          "options": {}
        }
      ]
    }
  ],
  "sources": {
    "org.osbuild.librepo": {}
  }
}`),
			ExpectedPipelines: &worker.PipelineNames{
				Build:   []string{"build"},
				Payload: []string{"os", "archive", "xz"},
			},
		},
		"unknown-build": {
			// The build property refers to a pipeline that doesn't exist. This
			// manifest is invalid, so it will fail to validate in osbuild, but
			// our function will work and ignore the invalid reference. This
			// means the "build-pipeline" will not be identified as a build
			// pipeline but instead be added to the Payload list along with
			// "root-tree".
			Manifest: []byte(`{
	"version": "2",
	"pipelines": [
		{
			"name": "build-pipeline"
		},
		{
			"name": "root-tree",
			"build": "build:not-a-pipeline"
		}
	],
	"sources": []
}`),
			ExpectedPipelines: &worker.PipelineNames{
				Payload: []string{"build-pipeline", "root-tree"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			plNames, err := main.ParseManifestPipelines(tc.Manifest)
			if tc.ExpectedError != "" {
				assert.EqualError(err, tc.ExpectedError)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.ExpectedPipelines, plNames)
		})
	}
}
