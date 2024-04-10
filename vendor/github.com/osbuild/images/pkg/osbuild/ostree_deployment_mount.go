package osbuild

import "github.com/osbuild/images/internal/common"

type OSTreeMountSource string

const (
	OSTreeMountSourceTree  OSTreeMountSource = "tree"
	OSTreeMountSourceMount OSTreeMountSource = "mount"
)

type OSTreeMountOptions struct {
	Source     OSTreeMountSource     `json:"source,omitempty"`
	Deployment OSTreeMountDeployment `json:"deployment"`
}

func (OSTreeMountOptions) isMountOptions() {}

type OSTreeMountDeployment struct {
	// Name of the stateroot to be used in the deployment
	OSName string `json:"osname,omitempty"`

	// OStree ref to create and use for deployment
	Ref string `json:"ref,omitempty"`

	// The deployment serial (usually '0')
	Serial *int `json:"serial,omitempty"`

	// When set the OSName/Ref/Serial is detected automatically
	Default *bool `json:"default,omitempty"`
}

func NewOSTreeDeploymentMount(name, osName, ref string, serial int) *Mount {
	return &Mount{
		Type: "org.osbuild.ostree.deployment",
		Name: name,
		Options: &OSTreeMountOptions{
			Deployment: OSTreeMountDeployment{
				OSName: osName,
				Ref:    ref,
				Serial: &serial,
			},
		},
	}
}

func NewOSTreeDeploymentMountDefault(name string, source OSTreeMountSource) *Mount {
	return &Mount{
		Type: "org.osbuild.ostree.deployment",
		Name: name,
		Options: &OSTreeMountOptions{
			Source: source,
			Deployment: OSTreeMountDeployment{
				Default: common.ToPtr(true),
			},
		},
	}
}
