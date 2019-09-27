package target

import "github.com/google/uuid"

type Target struct {
	Name    string  `json:"name"`
	Options Options `json:"options"`
}

type Options struct {
	Location string `json:"location"`
}

func New(ComposeID uuid.UUID) *Target {
	return &Target{
		Name: "org.osbuild.local",
		Options: Options{
			Location: "/var/lib/osbuild-composer/outputs/" + ComposeID.String(),
		},
	}
}
