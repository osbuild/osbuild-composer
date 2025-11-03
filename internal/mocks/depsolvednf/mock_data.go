package depsolvednf_mock

import (
	"fmt"
	"slices"
	"time"

	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/rpmmd"
)

// BaseFetchResult returns a mock list of packages for a repository.
// It contains 22 packages, package0 to package21. For each package,
// a second build is created with the version and build time incremented by 1.
// The returned list is ordered by package name and the version, i.e.:
// package0-0.0, package0-0.1, package1-1.0, package1-1.1, ...
//
// The return value is used for the FetchMetadata() and SearchMetadata() methods.
func BaseFetchResult() rpmmd.PackageList {
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

var DepsolvePackageNotExistError = depsolvednf.Error{
	Kind:   "MarkingErrors",
	Reason: "Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash",
}

var DepsolveBadError = depsolvednf.Error{
	Kind:   "DepsolveError",
	Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
}

var FetchError = depsolvednf.Error{
	Kind:   "FetchError",
	Reason: "There was a problem when fetching packages.",
}

// BaseSearchResultsMap creates results map for use with the Solver search command
// which is used for listing a subset of content from the repositories.
// The map key is a comma-separated list of the packages requested.
func BaseSearchResultsMap() map[string]rpmmd.PackageList {
	allPackages := BaseFetchResult()

	return map[string]rpmmd.PackageList{
		"":                  allPackages,
		"*":                 allPackages,
		"nonexistingpkg":    {},
		"package1":          allPackages[2:4],
		"package1,package2": allPackages[2:6],
		"package2*,package16": slices.Concat(
			allPackages[4:6],   // package2-2, package2-2.1
			allPackages[32:34], // package16-16, package16-16.1
			allPackages[40:44], // package20-20, package20-20.1, package21-21, package21-21.1
		),
		"package16": allPackages[32:34],
	}
}

// BaseDepsolveResult is the expected list of dependencies (as rpmmd.PackageList) from
func BaseDepsolveResult(repoID string) *depsolvednf.DepsolveResult {
	return &depsolvednf.DepsolveResult{
		Packages: rpmmd.PackageList{
			{
				Name:     "dep-package3",
				Epoch:    7,
				Version:  "3.0.3",
				Release:  "1.fc30",
				Arch:     "x86_64",
				CheckGPG: true,
				Checksum: rpmmd.Checksum{
					Type:  "sha256",
					Value: "62278d360aa5045eb202af39fe85743a4b5615f0c9c7439a04d75d785db4c720",
				},
				RemoteLocations: []string{
					"https://pkg3.example.com/3.0.3-1.fc30.x86_64.rpm",
				},
				RepoID: repoID,
			},
			{
				Name:     "dep-package1",
				Epoch:    0,
				Version:  "1.33",
				Release:  "2.fc30",
				Arch:     "x86_64",
				CheckGPG: true,
				Checksum: rpmmd.Checksum{
					Type:  "sha256",
					Value: "fe3951d112c3b1c84dc8eac57afe0830df72df1ca0096b842f4db5d781189893",
				},
				RemoteLocations: []string{
					"https://pkg1.example.com/1.33-2.fc30.x86_64.rpm",
				},
				RepoID: repoID,
			},
			{
				Name:     "dep-package2",
				Epoch:    0,
				Version:  "2.9",
				Release:  "1.fc30",
				Arch:     "x86_64",
				CheckGPG: true,
				Checksum: rpmmd.Checksum{
					Type:  "sha256",
					Value: "5797c0b0489681596b5b3cd7165d49870b85b69d65e08770946380a3dcd49ea2",
				},
				RemoteLocations: []string{
					"https://pkg2.example.com/2.9-1.fc30.x86_64.rpm",
				},
				RepoID: repoID,
			},
		},
		Repos: []rpmmd.RepoConfig{
			{
				Id:       repoID,
				BaseURLs: []string{"https://pkgs.example.com/"},
			},
		},
	}
}
