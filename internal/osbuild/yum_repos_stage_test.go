package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestNewYumReposStage(t *testing.T) {
	stageOptions := NewYumReposStageOptions("testing.repo", []YumRepository{
		{
			Id:       "cool-id",
			BaseURLs: []string{"http://example.org/repo"},
		},
	})
	expectedStage := &Stage{
		Type:    "org.osbuild.yum.repos",
		Options: stageOptions,
	}
	actualStage := NewYumReposStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestYumReposStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options YumReposStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: YumReposStageOptions{},
			err:     true,
		},
		{
			name: "no-repos",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos:    []YumRepository{},
			},
			err: true,
		},
		{
			name: "invalid-filename",
			options: YumReposStageOptions{
				Filename: "@#$%^&.rap",
				Repos: []YumRepository{
					{
						Id:       "cool-id",
						BaseURLs: []string{"http://example.org/repo"},
					},
				},
			},
			err: true,
		},
		{
			name: "no-filename",
			options: YumReposStageOptions{
				Repos: []YumRepository{
					{
						Id:       "cool-id",
						BaseURLs: []string{"http://example.org/repo"},
					},
				},
			},
			err: true,
		},
		{
			name: "no-baseurl-mirrorlist-metalink",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id: "cool-id",
					},
				},
			},
			err: true,
		},
		{
			name: "baseurl-empty-string",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:       "cool-id",
						BaseURLs: []string{""},
					},
				},
			},
			err: true,
		},
		{
			name: "gpgkey-empty-string",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:       "cool-id",
						BaseURLs: []string{"http://example.org/repo"},
						GPGKey:   []string{""},
					},
				},
			},
			err: true,
		},
		{
			name: "invalid-repo-id",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:       "c@@l-id",
						BaseURLs: []string{"http://example.org/repo"},
					},
				},
			},
			err: true,
		},
		{
			name: "good-options-baseurl",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:             "cool-id",
						Cost:           common.ToPtr(0),
						Enabled:        common.ToPtr(false),
						ModuleHotfixes: common.ToPtr(false),
						Name:           "c@@l-name",
						GPGCheck:       common.ToPtr(true),
						RepoGPGCheck:   common.ToPtr(true),
						BaseURLs:       []string{"http://example.org/repo"},
						GPGKey:         []string{"secretkey"},
					},
				},
			},
			err: false,
		},
		{
			name: "good-options-mirrorlist",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:             "cool-id",
						Cost:           common.ToPtr(200),
						Enabled:        common.ToPtr(true),
						ModuleHotfixes: common.ToPtr(true),
						Name:           "c@@l-name",
						GPGCheck:       common.ToPtr(false),
						RepoGPGCheck:   common.ToPtr(false),
						Mirrorlist:     "http://example.org/mirrorlist",
						GPGKey:         []string{"secretkey"},
					},
				},
			},
			err: false,
		},
		{
			name: "good-options-metalink",
			options: YumReposStageOptions{
				Filename: "test.repo",
				Repos: []YumRepository{
					{
						Id:       "cool-id",
						Metalink: "http://example.org/metalink",
					},
				},
			},
			err: false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewYumReposStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewYumReposStage(&tt.options) })
			}
		})
	}
}
