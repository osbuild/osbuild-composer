package runner

import "fmt"

type CentOS struct {
	Version uint64
}

func (c *CentOS) String() string {
	return fmt.Sprintf("org.osbuild.centos%d", c.Version)
}

func (c *CentOS) GetBuildPackages() []string {
	packages := []string{
		"glibc", // ldconfig
	}
	if c.Version >= 8 {
		packages = append(packages,
			"systemd", // systemd-tmpfiles and systemd-sysusers
		)
	}
	return packages
}
