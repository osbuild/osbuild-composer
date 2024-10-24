package common

import (
	"fmt"
)

// These constants are set during buildtime using additional
// compiler flags. Not all of them are necessarily defined
// because RPMs can be build from a tarball and spec file without
// being in a git repository. On the other hand when building
// composer inside of a container, there is no RPM layer so in
// that case the RPM version doesn't exist at all.
var (
	// Git revision from which this code was built
	GitRev = "undefined"

	// RPM Version
	RpmVersion = "undefined"
)

func BuildVersion() string {
	if GitRev != "undefined" {
		return fmt.Sprintf("git-rev:%s", GitRev)
	} else if RpmVersion != "undefined" {
		return fmt.Sprintf("NEVRA:%s", RpmVersion)
	} else {
		return "devel"
	}
}
