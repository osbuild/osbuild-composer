package osbuild

import (
	"github.com/osbuild/image-builder/pkg/container"
)

func GenContainerStorageStages(storagePath string, containerSpecs []container.Spec) (stages []*Stage) {
	return GenContainerStorageStagesWithMounts(storagePath, containerSpecs, nil, nil)
}

func GenContainerStorageStagesWithMounts(storagePath string, containerSpecs []container.Spec, devices map[string]Device, mounts []Mount) (stages []*Stage) {
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

	// Setup mounts on all stages if they are needed
	for _, s := range stages {
		if len(devices) > 0 {
			s.Devices = devices
		}
		if len(mounts) > 0 {
			s.Mounts = mounts
		}
	}

	return stages
}
