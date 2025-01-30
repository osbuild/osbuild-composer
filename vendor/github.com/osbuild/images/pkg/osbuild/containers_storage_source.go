package osbuild

import "fmt"

const SourceNameContainersStorage = "org.osbuild.containers-storage"

type ContainersStorageSource struct {
	Items map[string]struct{} `json:"items"`
}

func (ContainersStorageSource) isSource() {}

func NewContainersStorageSource() *ContainersStorageSource {
	return &ContainersStorageSource{
		Items: make(map[string]struct{}),
	}
}

func (source *ContainersStorageSource) AddItem(id string) {
	if !skopeoDigestPattern.MatchString(id) {
		panic(fmt.Errorf("item %#v has invalid image id", id))
	}
	source.Items[id] = struct{}{}
}
