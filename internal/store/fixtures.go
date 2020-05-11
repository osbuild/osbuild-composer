package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	test_distro "github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/target"
)

func FixtureBase() *Store {
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

	d := test_distro.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	imgType, err := arch.GetImageType("qcow2")
	if err != nil {
		panic("invalid image type qcow2 for x86_64 @ fedoratest")
	}
	s := New(nil, arch)

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): Compose{
			Blueprint: &b,
			ImageBuilds: []ImageBuild{
				{
					QueueStatus: common.IBWaiting,
					ImageType:   imgType,
					Targets:     []*target.Target{localTarget, awsTarget},
					JobCreated:  date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): Compose{
			Blueprint: &b,
			ImageBuilds: []ImageBuild{
				{
					QueueStatus: common.IBRunning,
					ImageType:   imgType,
					Targets:     []*target.Target{localTarget},
					JobCreated:  date,
					JobStarted:  date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000002"): Compose{
			Blueprint: &b,
			ImageBuilds: []ImageBuild{
				{
					QueueStatus: common.IBFinished,
					ImageType:   imgType,
					Targets:     []*target.Target{localTarget, awsTarget},
					JobCreated:  date,
					JobStarted:  date,
					JobFinished: date,
				},
			},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000003"): Compose{
			Blueprint: &b,
			ImageBuilds: []ImageBuild{
				{
					QueueStatus: common.IBFailed,
					ImageType:   imgType,
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

func FixtureEmpty() *Store {
	var bName = "test"
	var b = blueprint.Blueprint{
		Name:           bName,
		Version:        "0.0.0",
		Packages:       []blueprint.Package{},
		Modules:        []blueprint.Package{},
		Groups:         []blueprint.Group{},
		Customizations: nil,
	}

	d := test_distro.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	s := New(nil, arch)

	s.blueprints[bName] = b

	return s
}
