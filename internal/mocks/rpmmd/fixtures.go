package rpmmd_mock

import (
	"fmt"
	"sort"
	"time"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/compose"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
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
	var b = blueprint.Blueprint{
		Name:           bName,
		Version:        "0.0.0",
		Packages:       []blueprint.Package{},
		Modules:        []blueprint.Package{},
		Groups:         []blueprint.Group{},
		Customizations: nil,
	}

	var date = time.Date(2019, 11, 27, 13, 19, 0, 0, time.FixedZone("UTC+1", 60*60))

	var localTarget = &target.Target{
		Uuid:      uuid.MustParse("20000000-0000-0000-0000-000000000000"),
		Name:      "org.osbuild.local",
		ImageName: "localimage",
		Created:   date,
		Status:    common.IBWaiting,
		Options:   &target.LocalTargetOptions{},
	}

	var awsTarget = &target.Target{
		Uuid:      uuid.MustParse("10000000-0000-0000-0000-000000000000"),
		Name:      "org.osbuild.aws",
		ImageName: "awsimage",
		Created:   date,
		Status:    common.IBWaiting,
		Options: &target.AWSTargetOptions{
			Region:          "frankfurt",
			AccessKeyID:     "accesskey",
			SecretAccessKey: "secretkey",
			Bucket:          "clay",
			Key:             "imagekey",
		},
	}

	s := store.New(nil)

	s.Blueprints[bName] = b
	s.Composes = map[uuid.UUID]compose.Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): compose.Compose{
			Blueprint: &b,
			ImageBuilds: []compose.ImageBuild{
				{
					QueueStatus: common.IBWaiting,
					ImageType:   common.Qcow2Generic,
					Targets:     []*target.Target{localTarget, awsTarget},
					JobCreated:  date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): compose.Compose{
			Blueprint: &b,
			ImageBuilds: []compose.ImageBuild{
				{
					QueueStatus: common.IBRunning,
					ImageType:   common.Qcow2Generic,
					Targets:     []*target.Target{localTarget},
					JobCreated:  date,
					JobStarted:  date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000002"): compose.Compose{
			Blueprint: &b,
			ImageBuilds: []compose.ImageBuild{
				{
					QueueStatus: common.IBFinished,
					ImageType:   common.Qcow2Generic,
					Targets:     []*target.Target{localTarget, awsTarget},
					JobCreated:  date,
					JobStarted:  date,
					JobFinished: date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000003"): compose.Compose{
			Blueprint: &b,
			ImageBuilds: []compose.ImageBuild{
				{
					QueueStatus: common.IBFailed,
					ImageType:   common.Qcow2Generic,
					Targets:     []*target.Target{localTarget, awsTarget},
					JobCreated:  date,
					JobStarted:  date,
					JobFinished: date,
				},
			},
		},
	}

	return s
}

func createBaseDepsolveFixture() []rpmmd.PackageSpec {
	return []rpmmd.PackageSpec{
		{
			Name:    "dep-package3",
			Epoch:   7,
			Version: "3.0.3",
			Release: "1.fc30",
			Arch:    "x86_64",
		},
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
	}
}

func createStoreWithoutComposesFixture() *store.Store {
	var bName = "test"
	var b = blueprint.Blueprint{
		Name:           bName,
		Version:        "0.0.0",
		Packages:       []blueprint.Package{},
		Modules:        []blueprint.Package{},
		Groups:         []blueprint.Group{},
		Customizations: nil,
	}

	s := store.New(nil)

	s.Blueprints[bName] = b

	return s
}

func BaseFixture() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		depsolve{
			createBaseDepsolveFixture(),
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		createBaseStoreFixture(),
	}
}

func NoComposesFixture() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		depsolve{
			createBaseDepsolveFixture(),
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		createStoreWithoutComposesFixture(),
	}
}

func NonExistingPackage() Fixture {
	return Fixture{
		fetchPackageList{
			generatePackageList(),
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		depsolve{
			nil,
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
			map[string]string{"base": "sha256:f34848ca92665c342abd5816c9e3eda0e82180671195362bcd0080544a3bc2ac"},
			nil,
		},
		depsolve{
			nil,
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
			ret:       nil,
			checksums: nil,
			err: &rpmmd.DNFError{
				Kind:   "FetchError",
				Reason: "There was a problem when fetching packages.",
			},
		},
		depsolve{
			nil,
			nil,
			&rpmmd.DNFError{
				Kind:   "DepsolveError",
				Reason: "There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch",
			},
		},
		createBaseStoreFixture(),
	}
}
