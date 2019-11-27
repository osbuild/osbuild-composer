package rpmmd_mock

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"sort"
	"time"
)

type FixtureGenerator func() Fixture

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

	sort.Slice(packageList, func(i, j int) bool {
		return packageList[i].Name < packageList[j].Name
	})

	return packageList
}

func createBaseStoreFixture() *store.Store {
	var bName = "test"
	var b = blueprint.Blueprint{Name: bName, Version: "0.0.0"}

	var date = time.Date(2019, 11, 27, 13, 19, 0, 0, time.FixedZone("UTC+1", 60*60))

	s := store.New(nil)

	s.Blueprints[bName] = b
	s.Composes = map[uuid.UUID]store.Compose{
		uuid.MustParse("e65f76f8-b0d9-4974-9dd7-745ae80b4721"): store.Compose{
			QueueStatus: "WAITING",
			Blueprint:   &b,
			OutputType:  "tar",
			Targets:     nil,
			JobCreated:  date,
		},
		uuid.MustParse("e65f76f8-b0d9-4974-9dd7-745ae80b4722"): store.Compose{
			QueueStatus: "RUNNING",
			Blueprint:   &b,
			OutputType:  "tar",
			Targets:     nil,
			JobCreated:  date,
			JobStarted:  date,
		},
		uuid.MustParse("e65f76f8-b0d9-4974-9dd7-745ae80b4723"): store.Compose{
			QueueStatus: "FINISHED",
			Blueprint:   &b,
			OutputType:  "tar",
			Targets:     nil,
			JobCreated:  date,
			JobStarted:  date,
			JobFinished: date,
		},
		uuid.MustParse("e65f76f8-b0d9-4974-9dd7-745ae80b4724"): store.Compose{
			QueueStatus: "FAILED",
			Blueprint:   &b,
			OutputType:  "tar",
			Targets:     nil,
			JobCreated:  date,
			JobStarted:  date,
			JobFinished: date,
		},
	}

	return s
}

func BaseFixture() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			nil,
		},
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
		createBaseStoreFixture(),
	}
}

func NonExistingPackage() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			nil,
		},
		depsolve{
			nil,
			&rpmmd.DNFError{
				Kind:   "MarkingErrors",
				Reason: "Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash",
			},
		},
		createBaseStoreFixture(),
	}
}

func BadDepsolve() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			nil,
		},
		depsolve{
			nil,
			&rpmmd.DNFError{
				Kind:   "DepsolveError",
				Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
			},
		},
		createBaseStoreFixture(),
	}
}

func BadFetch() Fixture {
	return Fixture{
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
		createBaseStoreFixture(),
	}
}
