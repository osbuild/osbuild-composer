package dependencies

import (
	_ "embed"
)

//go:embed osbuild
var minimumOSBuildVersion string

// MinimumOSBuildVersion returns the minimum version of osbuild required by this
// version of images module.
func MinimumOSBuildVersion() string {
	return minimumOSBuildVersion
}
