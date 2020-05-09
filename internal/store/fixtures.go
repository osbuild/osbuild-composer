package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/compose"
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

	s := New(nil)

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]compose.Compose{
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

	s := New(nil)

	s.blueprints[bName] = b

	return s
}
