package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChownStage(t *testing.T) {
	stageOptions := &ChownStageOptions{
		Items: map[string]ChownStagePathOptions{
			"/etc/foobar": {
				User:      "root",
				Group:     int64(12345),
				Recursive: true,
			},
		},
	}
	expectedStage := &Stage{
		Type:    "org.osbuild.chown",
		Options: stageOptions,
	}
	actualStage := NewChownStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestChownStageOptionsValidate(t *testing.T) {
	validPathOptions := ChownStagePathOptions{
		User: "root",
	}

	testCases := []struct {
		name    string
		options *ChownStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: &ChownStageOptions{},
		},
		{
			name: "no-items",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{},
			},
		},
		{
			name: "invalid-item-path-1",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "invalid-item-path-2",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"foobar": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "invalid-item-path-3",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/../foobar": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "invalid-item-path-4",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/etc/../foobar": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "invalid-item-path-5",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/etc/..": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "invalid-item-path-6",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"../etc/foo/../bar": validPathOptions,
				},
			},
			err: true,
		},
		{
			name: "valid-item-path-1",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/etc/foobar": validPathOptions,
				},
			},
		},
		{
			name: "valid-item-path-2",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/etc/foo/bar/baz": validPathOptions,
				},
			},
		},
		{
			name: "valid-item-path-3",
			options: &ChownStageOptions{
				Items: map[string]ChownStagePathOptions{
					"/etc": validPathOptions,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.validate()
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChownStagePathOptionsValidate(t *testing.T) {
	testCases := []struct {
		name    string
		options ChownStagePathOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: ChownStagePathOptions{},
			err:     true,
		},
		{
			name: "invalid-user-string-1",
			options: ChownStagePathOptions{
				User: "",
			},
			err: true,
		},
		{
			name: "invalid-user-string-2",
			options: ChownStagePathOptions{
				User: "r@@t",
			},
			err: true,
		},
		{
			name: "invalid-user-id",
			options: ChownStagePathOptions{
				User: int64(-1),
			},
			err: true,
		},
		{
			name: "valid-user-string",
			options: ChownStagePathOptions{
				User: "root",
			},
		},
		{
			name: "valid-user-id",
			options: ChownStagePathOptions{
				User: int64(0),
			},
		},
		{
			name: "invalid-group-string-1",
			options: ChownStagePathOptions{
				Group: "",
			},
			err: true,
		},
		{
			name: "invalid-group-string-2",
			options: ChownStagePathOptions{
				Group: "r@@t",
			},
			err: true,
		},
		{
			name: "invalid-group-id",
			options: ChownStagePathOptions{
				Group: int64(-1),
			},
			err: true,
		},
		{
			name: "valid-group-string",
			options: ChownStagePathOptions{
				Group: "root",
			},
		},
		{
			name: "valid-group-id",
			options: ChownStagePathOptions{
				Group: int64(0),
			},
		},
		{
			name: "valid-both-1",
			options: ChownStagePathOptions{
				User:      "root",
				Group:     int64(12345),
				Recursive: true,
			},
		},
		{
			name: "valid-both-2",
			options: ChownStagePathOptions{
				User:      int64(12345),
				Group:     "root",
				Recursive: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.validate()
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
