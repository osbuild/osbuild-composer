package osbuild

import (
	"github.com/osbuild/images/pkg/container"
)

func GenContainerStorageStages(storagePath string, containerSpecs []container.Spec) (stages []*Stage) {
	if storagePath != "" {
		storageConf := "/etc/containers/storage.conf"

		containerStoreOpts := NewContainerStorageOptions(storageConf, storagePath)
		stages = append(stages, NewContainersStorageConfStage(containerStoreOpts))
	}

	images := NewContainersInputForSources(containerSpecs)
	localImages := NewLocalContainersInputForSources(containerSpecs)

	if len(images.References) > 0 {
		manifests := NewFilesInputForManifestLists(containerSpecs)
		stages = append(stages, NewSkopeoStageWithContainersStorage(storagePath, images, manifests))
	}

	if len(localImages.References) > 0 {
		stages = append(stages, NewSkopeoStageWithContainersStorage(storagePath, localImages, nil))

	}

	return stages
}
