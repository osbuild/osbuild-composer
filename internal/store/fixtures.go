package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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

	d := fedoratest.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	imgType, err := arch.GetImageType("qcow2")
	if err != nil {
		panic("invalid image type qcow2 for x86_64 @ fedoratest")
	}
	manifest, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil, 0)
	if err != nil {
		panic("could not create manifest")
	}
	s := New(nil, arch, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    "x86_64",
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    "x86_64",
		}}

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBWaiting,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBRunning,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget},
				JobCreated:  date,
				JobStarted:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000002"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000003"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFailed,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000004"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: pkgs,
		},
	}

	return s
}
func FixtureFinished() *Store {
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

	d := fedoratest.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	imgType, err := arch.GetImageType("qcow2")
	if err != nil {
		panic("invalid image type qcow2 for x86_64 @ fedoratest")
	}
	manifest, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil, 0)
	if err != nil {
		panic("could not create manifest")
	}
	s := New(nil, arch, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    "x86_64",
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    "x86_64",
		}}

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget},
				JobCreated:  date,
				JobStarted:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000003"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFailed,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget, awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000004"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    manifest,
				Targets:     []*target.Target{localTarget},
				JobCreated:  date,
				JobStarted:  date,
			},
			Packages: pkgs,
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

	d := fedoratest.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	s := New(nil, arch, nil)

	s.blueprints[bName] = b

	return s
}
