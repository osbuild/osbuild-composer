package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
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

	d := test_distro.New()
	arch, err := d.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", test_distro.TestArchName, err))
	}
	imgType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(fmt.Sprintf("failed to get image type %s for a test distro architecture: %v", test_distro.TestImageTypeName, err))
	}
	manifest, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil, 0)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}
	s := New(nil, arch, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    test_distro.TestArchName,
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    test_distro.TestArchName,
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

	d := test_distro.New()
	arch, err := d.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", test_distro.TestArchName, err))
	}
	imgType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(fmt.Sprintf("failed to get image type %s for a test distro architecture: %v", test_distro.TestImageTypeName, err))
	}
	manifest, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil, 0)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}
	s := New(nil, arch, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    test_distro.TestArchName,
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    test_distro.TestArchName,
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

	d := test_distro.New()
	arch, err := d.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", test_distro.TestArchName, err))
	}

	s := New(nil, arch, nil)

	s.blueprints[bName] = b

	// 2nd distro blueprint
	b2 := b
	b2.Name = "test-distro-2"
	b2.Distro = "test-distro-2"
	s.blueprints[b2.Name] = b2

	// Unknown distro blueprint
	b3 := b
	b3.Name = "test-fedora-1"
	b3.Distro = "fedora-1"
	s.blueprints[b3.Name] = b3

	return s
}
