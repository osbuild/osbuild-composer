package osbuild

type SkopeoDestination struct {
	Type          string `json:"type"`
	StoragePath   string `json:"storage-path,omitempty"`
	StorageDriver string `json:"sotrage-driver,omitempty"`
}

type SkopeoStageOptions struct {
	Destination SkopeoDestination `json:"destination"`
}

func (o SkopeoStageOptions) isStageOptions() {}

func NewSkopeoStage(images ContainersInput, path string) *Stage {

	inputs := ContainersInputs{
		"images": images,
	}

	return &Stage{
		Type: "org.osbuild.skopeo",
		Options: &SkopeoStageOptions{
			Destination: SkopeoDestination{
				Type:        "containers-storage",
				StoragePath: path,
			},
		},
		Inputs: inputs,
	}
}
