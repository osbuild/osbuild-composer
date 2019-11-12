package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

var BaseFixture = Fixture{
	fetchPackageList{
		rpmmd.PackageList{
			{Name: "package1"},
			{Name: "package2"},
		},
		nil,
	},
	depsolve{
		nil,
		nil,
	},
}
