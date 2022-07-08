package runner

import "fmt"

type RHEL struct {
	Major uint64
	Minor uint64
}

func (r *RHEL) String() string {
	return fmt.Sprintf("org.osbuild.fedora%d%d", r.Major, r.Minor)
}

func (p *RHEL) GetBuildPackages() []string {
	packages := []string{
		"glibc", // ldconfig
	}
	if p.Major >= 8 {
		packages = append(packages,
			"systemd", // systemd-tmpfiles and systemd-sysusers
		)
	}
	return packages
}
