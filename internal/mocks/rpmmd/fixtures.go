package rpmmd_mock

import (
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"time"
)

func generatePackageList() rpmmd.PackageList {
	baseTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

	if err != nil {
		panic(err)
	}

	var packageList rpmmd.PackageList

	for i := 0; i < 22; i++ {
		basePackage := rpmmd.Package{
			Name:        fmt.Sprintf("package%d", i),
			Summary:     fmt.Sprintf("pkg%d sum", i),
			Description: fmt.Sprintf("pkg%d desc", i),
			URL:         fmt.Sprintf("https://pkg%d.example.com", i),
			Epoch:       0,
			Version:     fmt.Sprintf("%d.0", i),
			Release:     fmt.Sprintf("%d.fc30", i),
			Arch:        "x86_64",
			BuildTime:   baseTime.AddDate(0, i, 0),
			License:     "MIT",
		}

		secondBuild := basePackage

		secondBuild.Version = fmt.Sprintf("%d.1", i)
		secondBuild.BuildTime = basePackage.BuildTime.AddDate(0, 0, 1)

		packageList = append(packageList, basePackage, secondBuild)
	}

	return packageList
}

var basePackageList = fetchPackageList{
	generatePackageList(),
	nil,
}

var BaseFixture = Fixture{
	basePackageList,
	depsolve{
		[]rpmmd.PackageSpec{
			{
				Name:    "dep-package1",
				Epoch:   0,
				Version: "1.33",
				Release: "2.fc30",
				Arch:    "x86_64",
			},
			{
				Name:    "dep-package2",
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

var BadFetch = Fixture{
	fetchPackageList{
		ret: nil,
		err: &rpmmd.DNFError{
			Kind:   "FetchError",
			Reason: "There was a problem when fetching packages.",
		},
	},
	depsolve{
		nil,
		&rpmmd.DNFError{
			Kind:   "DepsolveError",
			Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
		},
	},
}
