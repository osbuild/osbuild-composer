package common

import (
	"fmt"
	"runtime/debug"
)

const (
	OSBuildImagesModulePath = "github.com/osbuild/images"
)

// GetDepModuleInfoByPath returns the debug.Module for the dependency
// with the given path. If the dependency is not found, an error is
// returned.
func GetDepModuleInfoByPath(path string) (*debug.Module, error) {
	buildinfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fmt.Errorf("Failed to read build info")
	}

	for _, dep := range buildinfo.Deps {
		if dep.Path == path {
			return dep, nil
		}
	}

	return nil, fmt.Errorf("Could not find dependency %s", path)
}
