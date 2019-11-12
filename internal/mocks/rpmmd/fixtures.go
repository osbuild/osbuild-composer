package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

var basePackageList = fetchPackageList{
	rpmmd.PackageList{
		{Name: "package1"},
		{Name: "package2"},
	},
	nil,
}

var BaseFixture = Fixture{
	basePackageList,
	depsolve{
		[]rpmmd.PackageSpec{
			{
				Name:    "libgpg-error",
				Epoch:   0,
				Version: "1.33",
				Release: "2.fc30",
				Arch:    "x86_64",
			},
			{
				Name:    "libsemanage",
				Epoch:   0,
				Version: "2.9",
				Release: "1.fc30",
				Arch:    "x86_64",
			},
		},
		nil,
	},
}

var NonExistingPackage = Fixture{
	basePackageList,
	depsolve{
		nil,
		&rpmmd.DNFError{
			Kind:   "MarkingErrors",
			Reason: "Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash",
		},
	},
}

var BadDepsolve = Fixture{
	basePackageList,
	depsolve{
		nil,
		&rpmmd.DNFError{
			Kind:   "DepsolveError",
			Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
		},
	},
}
