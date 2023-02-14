package rhel8

import "github.com/osbuild/osbuild-composer/internal/workload"

// rhel8Workload is a RHEL-8-specific implementation of the workload interface
// for internal workload variants.
type rhel8Workload struct {
	workload.BaseWorkload
	packages []string
}

func (w rhel8Workload) GetPackages() []string {
	return w.packages
}

func eapWorkload() workload.Workload {
	w := rhel8Workload{}
	w.packages = []string{
		"java-1.8.0-openjdk",
		"java-1.8.0-openjdk-devel",
		"eap7-wildfly",
		"eap7-artemis-native-wildfly",
	}

	return &w
}
