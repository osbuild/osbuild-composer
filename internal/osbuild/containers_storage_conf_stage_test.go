package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainersStorageConfStage(t *testing.T) {
	expectedStage := &Stage{
		Type: "org.osbuild.containers.storage.conf",
		Options: &ContainersStorageConfStageOptions{
			Filename: "/usr/share/containers/storage.conf",
			Config: ContainersStorageConfig{
				Storage: CSCStorage{
					Options: &CSCStorageOptions{
						AdditionalImageStores: []string{
							"/usr/share/containers/storage/",
						},
					},
				},
			},
		},
	}

	actualStage := NewContainersStorageConfStage(
		NewContainerStorageOptions("/usr/share/containers/storage.conf",
			"/usr/share/containers/storage/"),
	)
	assert.Equal(t, expectedStage, actualStage)
}
