// dnfjson_mock provides data and methods for testing the dnfjson package.
package dnfjson_mock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/osbuild/images/pkg/dnfjson"
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

var DepsolvePackageNotExistError = dnfjson.Error{
	Kind:   "MarkingErrors",
	Reason: "Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash",
}

var DepsolveBadError = dnfjson.Error{
	Kind:   "DepsolveError",
	Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
}

var FetchError = dnfjson.Error{
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

// generateSearchResults creates results for use with the dnfjson search command
// which is used for listing a subset of modules and projects.
//
// The map key is a comma-separated list of the packages requested
// If no packages are included it returns all 22 packages, same as the mock dump
//
// nonexistingpkg returns an empty list
// badpackage1 returns a fetch error, same as when the package name is unknown
// baddepsolve returns package1, the test then tries to depsolve package1 using BadDepsolve()
// wich will return a depsolve error.
func generateSearchResults() map[string]interface{} {
	allPackages := BaseFetchResult()

	// This includes package16, package2, package20, and package21
	var wildcardResults rpmmd.PackageList
	wildcardResults = append(wildcardResults, allPackages[32], allPackages[33])
	wildcardResults = append(wildcardResults, allPackages[4], allPackages[5])
	for i := 40; i < 44; i++ {
		wildcardResults = append(wildcardResults, allPackages[i])
	}

	return map[string]interface{}{
		"":                    allPackages,
		"*":                   allPackages,
		"nonexistingpkg":      rpmmd.PackageList{},
		"package1":            rpmmd.PackageList{allPackages[2], allPackages[3]},
		"package1,package2":   rpmmd.PackageList{allPackages[2], allPackages[3], allPackages[4], allPackages[5]},
		"package2*,package16": wildcardResults,
		"package16":           rpmmd.PackageList{allPackages[32], allPackages[33]},
		"badpackage1":         FetchError,
		"baddepsolve":         rpmmd.PackageList{allPackages[2], allPackages[3]},
	}
}

// These are duplicated from images/pk/dnfjson
type depsolveResult struct {
	Packages []dnfjson.PackageSpec `json:"packages"`
	Repos    map[string]repoConfig `json:"repos"`
}

type repoConfig struct {
	ID       string `json:"id"`
	GPGCheck bool   `json:"gpgcheck"`
}

func createBaseDepsolveFixture() depsolveResult {
	return depsolveResult{
		Packages: []dnfjson.PackageSpec{
			{
				Name:     "dep-package3",
				Epoch:    7,
				Version:  "3.0.3",
				Release:  "1.fc30",
				Arch:     "x86_64",
				RepoID:   "REPOID", // added by mock-dnf-json
				Checksum: "sha256:62278d360aa5045eb202af39fe85743a4b5615f0c9c7439a04d75d785db4c720",
			},
			{
				Name:     "dep-package1",
				Epoch:    0,
				Version:  "1.33",
				Release:  "2.fc30",
				Arch:     "x86_64",
				RepoID:   "REPOID", // added by mock-dnf-json
				Checksum: "sha256:fe3951d112c3b1c84dc8eac57afe0830df72df1ca0096b842f4db5d781189893",
			},
			{
				Name:     "dep-package2",
				Epoch:    0,
				Version:  "2.9",
				Release:  "1.fc30",
				Arch:     "x86_64",
				RepoID:   "REPOID", // added by mock-dnf-json
				Checksum: "sha256:5797c0b0489681596b5b3cd7165d49870b85b69d65e08770946380a3dcd49ea2",
			},
		},
		Repos: map[string]repoConfig{
			"REPOID": repoConfig{
				ID:       "REPOID",
				GPGCheck: true,
			},
		},
	}
}

// BaseDepsolveResult is the expected list of dependencies (as rpmmd.PackageSpec) from
// the Base ResponseGenerator
func BaseDepsolveResult(repoID string) []rpmmd.PackageSpec {
	return []rpmmd.PackageSpec{
		{
			Name:     "dep-package3",
			Epoch:    7,
			Version:  "3.0.3",
			Release:  "1.fc30",
			Arch:     "x86_64",
			CheckGPG: true,
			Checksum: "sha256:62278d360aa5045eb202af39fe85743a4b5615f0c9c7439a04d75d785db4c720",
			RepoID:   repoID,
		},
		{
			Name:     "dep-package1",
			Epoch:    0,
			Version:  "1.33",
			Release:  "2.fc30",
			Arch:     "x86_64",
			CheckGPG: true,
			Checksum: "sha256:fe3951d112c3b1c84dc8eac57afe0830df72df1ca0096b842f4db5d781189893",
			RepoID:   repoID,
		},
		{
			Name:     "dep-package2",
			Epoch:    0,
			Version:  "2.9",
			Release:  "1.fc30",
			Arch:     "x86_64",
			CheckGPG: true,
			Checksum: "sha256:5797c0b0489681596b5b3cd7165d49870b85b69d65e08770946380a3dcd49ea2",
			RepoID:   repoID,
		},
	}
}

type ResponseGenerator func(string) string

func Base(tmpdir string) string {
	data := map[string]interface{}{
		"depsolve": createBaseDepsolveFixture(),
		"dump":     BaseFetchResult(),
		"search":   generateSearchResults(),
	}
	path := filepath.Join(tmpdir, "base.json")
	write(data, path)
	return path
}

func NonExistingPackage(tmpdir string) string {
	deps := dnfjson.Error{
		Kind:   "MarkingErrors",
		Reason: "Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash",
	}
	data := map[string]interface{}{
		"depsolve": deps,
	}
	path := filepath.Join(tmpdir, "notexist.json")
	write(data, path)
	return path
}

func BadDepsolve(tmpdir string) string {
	deps := dnfjson.Error{
		Kind:   "DepsolveError",
		Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
	}

	data := map[string]interface{}{
		"depsolve": deps,
		"dump":     BaseFetchResult(),
		"search":   generateSearchResults(),
	}
	path := filepath.Join(tmpdir, "baddepsolve.json")
	write(data, path)
	return path
}

func BadFetch(tmpdir string) string {
	deps := dnfjson.Error{
		Kind:   "DepsolveError",
		Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
	}
	pkgs := dnfjson.Error{
		Kind:   "FetchError",
		Reason: "There was a problem when fetching packages.",
	}
	data := map[string]interface{}{
		"depsolve": deps,
		"dump":     pkgs,
		"search":   generateSearchResults(),
	}
	path := filepath.Join(tmpdir, "badfetch.json")
	write(data, path)
	return path
}

func marshal(data interface{}) []byte {
	jdata, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return jdata
}

func write(data interface{}, path string) {
	fp, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if _, err := fp.Write(marshal(data)); err != nil {
		panic(err)
	}
}
