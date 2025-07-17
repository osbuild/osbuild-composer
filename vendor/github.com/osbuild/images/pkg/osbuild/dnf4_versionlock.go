package osbuild

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rpmmd"
)

const dnf4VersionlockType = "org.osbuild.dnf4.versionlock"

type DNF4VersionlockOptions struct {
	Add []string `json:"add"`
}

func (*DNF4VersionlockOptions) isStageOptions() {}

func (o *DNF4VersionlockOptions) validate() error {
	if len(o.Add) == 0 {
		return fmt.Errorf("%s: at least one package must be included in the 'add' list", dnf4VersionlockType)
	}

	return nil
}

func NewDNF4VersionlockStage(options *DNF4VersionlockOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    dnf4VersionlockType,
		Options: options,
	}
}

// GenDNF4VersionlockStageOptions creates DNF4VersionlockOptions for the provided
// packages at the specific EVR that is contained in the package spec list.
// Returns an error if:
//   - Any of the package names does not appear in the package specs.
//   - dnf4 is not in the package specs: we only support the feature in dnf4 for now.
//   - The python3-dnf-plugin-versionlock package is not in the package specs:
//     the plugin is required for the lock to be effective.
func GenDNF4VersionlockStageOptions(lockPackageNames []string, packageSpecs []rpmmd.PackageSpec) (*DNF4VersionlockOptions, error) {

	// check that dnf4 and the plugin are included in the package specs
	dnf, err := rpmmd.GetPackage(packageSpecs, "dnf")
	if err != nil {
		return nil, fmt.Errorf("%s: dnf version locking enabled for an image that does not contain dnf: %w", dnf4VersionlockType, err)
	}
	if common.VersionGreaterThanOrEqual(dnf.Version, "5") {
		return nil, fmt.Errorf("%s: dnf version locking enabled for an image that includes dnf version %s: the feature requires dnf4", dnf4VersionlockType, dnf.Version)
	}
	if _, err := rpmmd.GetPackage(packageSpecs, "python3-dnf-plugin-versionlock"); err != nil {
		return nil, fmt.Errorf("%s: dnf version locking enabled for an image that does not contain the versionlock plugin: %w", dnf4VersionlockType, err)
	}

	pkgNEVRs := make([]string, len(lockPackageNames))
	for idx, pkgName := range lockPackageNames {
		pkg, err := rpmmd.GetPackage(packageSpecs, pkgName)
		if err != nil {
			return nil, fmt.Errorf("%s: package %q not found in package list", dnf4VersionlockType, pkgName)
		}
		nevr := fmt.Sprintf("%s-%d:%s-%s", pkg.Name, pkg.Epoch, pkg.Version, pkg.Release)
		pkgNEVRs[idx] = nevr
	}

	return &DNF4VersionlockOptions{
		Add: pkgNEVRs,
	}, nil
}
