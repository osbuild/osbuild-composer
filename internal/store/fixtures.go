package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

func setupTestHostDistro(distroName, archName string) (Cleanup func()) {
	originalHostDistroNameFunc := getHostDistroName
	originalHostArchFunc := getHostArch

	getHostDistroName = func() (string, error) {
		return distroName, nil
	}

	getHostArch = func() string {
		return archName
	}

	return func() {
		getHostDistroName = originalHostDistroNameFunc
		getHostArch = originalHostArchFunc
	}
}

type Fixture struct {
	*Store
	Cleanup func()
	*distrofactory.Factory
	HostDistroName string
	HostArchName   string
}

func FixtureBase(hostDistroName, hostArchName string) *Fixture {
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

	var awsTarget = &target.Target{
		Uuid:      uuid.MustParse("10000000-0000-0000-0000-000000000000"),
		Name:      target.TargetNameAWS,
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

	df := distrofactory.NewTestDefault()
	d := df.GetDistro(hostDistroName)
	if d == nil {
		panic(fmt.Sprintf("failed to get distro object for distro name %q", hostDistroName))
	}
	arch, err := d.GetArch(hostArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", hostArchName, err))
	}
	imgType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(fmt.Sprintf("failed to get image type %s for a test distro architecture: %v", test_distro.TestImageTypeName, err))
	}
	manifest, _, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	s := New(nil, df, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}}

	s.blueprints[bName] = b

	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBWaiting,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
				JobCreated:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBRunning,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: pkgs,
		},
	}

	return &Fixture{
		s,
		setupTestHostDistro(hostDistroName, hostArchName),
		df,
		hostDistroName,
		hostArchName,
	}
}

func FixtureFinished(hostDistroName, hostArchName string) *Fixture {
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

	var gcpTarget = &target.Target{
		Uuid:      uuid.MustParse("20000000-0000-0000-0000-000000000000"),
		Name:      target.TargetNameGCP,
		ImageName: "localimage",
		Created:   date,
		Status:    common.IBWaiting,
		Options:   &target.GCPTargetOptions{},
	}

	var awsTarget = &target.Target{
		Uuid:      uuid.MustParse("10000000-0000-0000-0000-000000000000"),
		Name:      target.TargetNameAWS,
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

	df := distrofactory.NewTestDefault()
	d := df.GetDistro(hostDistroName)
	arch, err := d.GetArch(hostArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", hostArchName, err))
	}
	imgType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(fmt.Sprintf("failed to get image type %s for a test distro architecture: %v", test_distro.TestImageTypeName, err))
	}
	manifest, _, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	s := New(nil, df, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}}

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{gcpTarget, awsTarget},
				JobCreated:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{gcpTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{gcpTarget, awsTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{gcpTarget},
				JobCreated:  date,
				JobStarted:  date,
			},
			Packages: pkgs,
		},
	}

	return &Fixture{
		s,
		setupTestHostDistro(hostDistroName, hostArchName),
		df,
		hostDistroName,
		hostArchName,
	}
}

func FixtureEmpty(hostDistroName, hostArchName string) *Fixture {
	var bName = "test"
	var b = blueprint.Blueprint{
		Name:           bName,
		Version:        "0.0.0",
		Packages:       []blueprint.Package{},
		Modules:        []blueprint.Package{},
		Groups:         []blueprint.Group{},
		Customizations: nil,
	}

	df := distrofactory.NewTestDefault()
	d := df.GetDistro(hostDistroName)
	if d == nil {
		panic(fmt.Sprintf("failed to get distro object for distro name %q", hostDistroName))
	}
	_, err := d.GetArch(hostArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", hostArchName, err))
	}

	s := New(nil, df, nil)

	s.blueprints[bName] = b

	// 2nd distro blueprint
	b2 := b
	b2.Name = "test-distro-2"
	b2.Distro = fmt.Sprintf("%s-2", test_distro.TestDistroNameBase)
	s.blueprints[b2.Name] = b2

	// Unknown distro blueprint
	b3 := b
	b3.Name = "test-fedora-1"
	b3.Distro = "fedora-1"
	s.blueprints[b3.Name] = b3

	// Bad arch blueprint
	b4 := b
	b4.Name = "test-badarch"
	b4.Arch = "badarch"
	s.blueprints[b4.Name] = b4

	// Cross arch blueprint
	b5 := b
	b5.Name = "test-crossarch"
	b5.Arch = test_distro.TestArch2Name
	s.blueprints[b5.Name] = b5

	return &Fixture{
		s,
		setupTestHostDistro(hostDistroName, hostArchName),
		df,
		hostDistroName,
		hostArchName,
	}
}

// FixtureOldChanges contains a blueprint and old changes
// This simulates restarting the service and losing the old blueprints
func FixtureOldChanges(hostDistroName, hostArchName string) *Fixture {
	var bName = "test-old-changes"
	var b = blueprint.Blueprint{
		Name:           bName,
		Version:        "0.0.0",
		Packages:       []blueprint.Package{},
		Modules:        []blueprint.Package{},
		Groups:         []blueprint.Group{},
		Customizations: nil,
	}

	df := distrofactory.NewTestDefault()
	d := df.GetDistro(hostDistroName)
	_, err := d.GetArch(hostArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", hostArchName, err))
	}

	s := New(nil, df, nil)

	s.PushBlueprint(b, "Initial commit")
	b.Version = "0.0.1"
	b.Packages = []blueprint.Package{{Name: "tmux", Version: "1.2.3"}}
	s.PushBlueprint(b, "Add tmux package")
	b.Version = "0.0.2"
	b.Packages = []blueprint.Package{{Name: "tmux", Version: "*"}}
	s.PushBlueprint(b, "Change tmux version")

	// Replace the associated blueprints. This simulates reading the store from
	// disk which doesn't actually save the old blueprints to disk.
	for bp := range s.blueprintsChanges {
		for c := range s.blueprintsChanges[bp] {
			change := s.blueprintsChanges[bp][c]
			change.Blueprint = blueprint.Blueprint{}
			s.blueprintsChanges[bp][c] = change
		}
	}

	return &Fixture{
		s,
		setupTestHostDistro(hostDistroName, hostArchName),
		df,
		hostDistroName,
		hostArchName,
	}
}

// Fixture to use for checking job queue files
func FixtureJobs(hostDistroName, hostArchName string) *Fixture {
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

	var awsTarget = &target.Target{
		Uuid:      uuid.MustParse("10000000-0000-0000-0000-000000000000"),
		Name:      target.TargetNameAWS,
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

	df := distrofactory.NewTestDefault()
	d := df.GetDistro(hostDistroName)
	arch, err := d.GetArch(hostArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", hostArchName, err))
	}
	imgType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(fmt.Sprintf("failed to get image type %s for a test distro architecture: %v", test_distro.TestImageTypeName, err))
	}
	manifest, _, err := imgType.Manifest(nil, distro.ImageOptions{}, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create a manifest: %v", err))
	}

	s := New(nil, df, nil)

	pkgs := []rpmmd.PackageSpec{
		{
			Name:    "test1",
			Epoch:   0,
			Version: "2.11.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}, {
			Name:    "test2",
			Epoch:   3,
			Version: "4.2.2",
			Release: "1.fc35",
			Arch:    hostArchName,
		}}

	s.blueprints[bName] = b
	s.composes = map[uuid.UUID]Compose{
		uuid.MustParse("30000000-0000-0000-0000-000000000000"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBWaiting,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
				JobCreated:  date,
			},
			Packages: []rpmmd.PackageSpec{},
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000001"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBRunning,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
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
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
			},
			Packages: pkgs,
		},
		uuid.MustParse("30000000-0000-0000-0000-000000000005"): {
			Blueprint: &b,
			ImageBuild: ImageBuild{
				QueueStatus: common.IBFinished,
				ImageType:   imgType,
				Manifest:    mf,
				Targets:     []*target.Target{awsTarget},
				JobCreated:  date,
				JobStarted:  date,
				JobFinished: date,
				JobID:       uuid.MustParse("30000000-0000-0000-0000-000000000005"),
			},
			Packages: pkgs,
		},
	}

	return &Fixture{
		s,
		setupTestHostDistro(hostDistroName, hostArchName),
		df,
		hostDistroName,
		hostArchName,
	}
}
