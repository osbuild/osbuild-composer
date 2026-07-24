package runner

import "fmt"

type Ol struct {
	Version uint64
}

func (o *Ol) String() string {
	return fmt.Sprintf("org.osbuild.ol%d", o.Version)
}

func (o *Ol) GetBuildPackages() []string {
	packages := []string{
		"glibc",           // ldconfig
		"platform-python", // osbuild
		"python3",
	}
	return packages
}
