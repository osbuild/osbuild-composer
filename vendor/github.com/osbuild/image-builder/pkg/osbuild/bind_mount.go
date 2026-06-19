package osbuild

import (
	"fmt"
	"strings"
)

type BindMountOptions struct {
	Source string `json:"source,omitempty"`
}

func (BindMountOptions) isMountOptions() {}

func NewBindMount(name, source, target string) *Mount {
	if !strings.HasPrefix(source, "mount://") {
		panic(fmt.Errorf(`bind mount source must start with "mount://", got %q`, source))
	}
	if !strings.HasPrefix(target, "tree://") {
		panic(fmt.Errorf(`bind mount target must start with "tree://", got %q`, target))
	}

	return &Mount{
		Type:   "org.osbuild.bind",
		Name:   name,
		Target: target,
		Options: &BindMountOptions{
			Source: source,
		},
	}
}
