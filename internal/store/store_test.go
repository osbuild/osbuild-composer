package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

// struct for sharing state between tests
type storeTest struct {
	suite.Suite
	dir              string
	myStore          *Store
	myCustomizations blueprint.Customizations
	myBP             blueprint.Blueprint
	myBPv3           blueprint.Blueprint
	CommitHash       []string
	myChange         []blueprint.Change
	myTarget         *target.Target
	mySources        map[string]osbuild.Source
	myCompose        Compose
	myImageBuild     ImageBuild
	mySourceConfig   SourceConfig
	myDistro         distro.Distro
	myArch           distro.Arch
	myImageType      distro.ImageType
	myManifest       manifest.OSBuildManifest
	myRepoConfig     []rpmmd.RepoConfig
	myPackageSpec    []rpmmd.PackageSpec
	myImageOptions   distro.ImageOptions
	myPackages       []rpmmd.PackageSpec
}

// func to initialize some default values before the suite is ran
func (suite *storeTest) SetupSuite() {
	var err error
	suite.myRepoConfig = []rpmmd.RepoConfig{rpmmd.RepoConfig{
		Name:       "testRepo",
		MirrorList: "testURL",
	}}
	suite.myPackageSpec = []rpmmd.PackageSpec{rpmmd.PackageSpec{}}
	suite.myDistro = test_distro.DistroFactory(test_distro.TestDistro1Name)
	suite.NotNil(suite.myDistro)
	suite.myArch, err = suite.myDistro.GetArch(test_distro.TestArchName)
	suite.NoError(err)
	suite.myImageType, err = suite.myArch.GetImageType(test_distro.TestImageTypeName)
	suite.NoError(err)
	ibp := blueprint.Convert(suite.myBP)
	manifest, _, _ := suite.myImageType.Manifest(&ibp, suite.myImageOptions, suite.myRepoConfig, nil)
	suite.myManifest, _ = manifest.Serialize(nil, nil, nil, nil)
	suite.mySourceConfig = SourceConfig{
		Name: "testSourceConfig",
	}
	suite.myPackages = []rpmmd.PackageSpec{
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
	suite.myCompose = Compose{
		Blueprint:  &suite.myBP,
		ImageBuild: suite.myImageBuild,
		Packages:   suite.myPackages,
	}
	suite.myImageBuild = ImageBuild{
		ID: 123,
	}
	suite.mySources = make(map[string]osbuild.Source)
	suite.myCustomizations = blueprint.Customizations{}
	suite.myBP = blueprint.Blueprint{
		Name:        "testBP",
		Description: "Testing blueprint",
		Version:     "0.0.1",
		Packages: []blueprint.Package{
			{Name: "test1", Version: "*"}},
		Modules: []blueprint.Package{
			{Name: "test2", Version: "*"}},
		Groups: []blueprint.Group{
			{Name: "test3"}},
		Containers: []blueprint.Container{
			{
				Source:    "https://registry.example.com/container",
				Name:      "example-container",
				TLSVerify: common.ToPtr(true),
			},
		},
		Customizations: &suite.myCustomizations,
	}
	suite.myBPv3 = blueprint.Blueprint{
		Name:        "testBP",
		Description: "Testing tagging testBP blueprint",
		Version:     "3.0.0",
		Packages: []blueprint.Package{
			{Name: "test4", Version: "*"}},
		Modules: []blueprint.Package{
			{Name: "test5", Version: "*"}},
		Groups: []blueprint.Group{
			{Name: "test6"}},
		Customizations: &suite.myCustomizations,
	}
	suite.CommitHash = []string{"firstCommit", "secondCommit"}
	suite.myChange = []blueprint.Change{
		blueprint.Change{
			Commit:    "firstCommit",
			Message:   "firstCommitMessage",
			Revision:  nil,
			Timestamp: "now",
			Blueprint: suite.myBP,
		},
		blueprint.Change{
			Commit:    "secondCommit",
			Message:   "secondCommitMessage",
			Revision:  nil,
			Timestamp: "now",
			Blueprint: suite.myBPv3,
		},
	}
	suite.myTarget = &target.Target{
		Uuid:      uuid.New(),
		ImageName: "ImageName",
		Name:      "Name",
		Created:   time.Now(),
		Options:   nil,
	}

}

// setup before each test
func (suite *storeTest) SetupTest() {
	suite.dir = suite.T().TempDir()
	df := distrofactory.NewTestDefault()
	suite.myStore = New(&suite.dir, df, nil)
}

func (suite *storeTest) TestRandomSHA1String() {
	hash, err := randomSHA1String()
	suite.NoError(err)
	suite.Len(hash, 40)
}

// Check initial state of fields
func (suite *storeTest) TestNewEmpty() {
	suite.Empty(suite.myStore.blueprints)
	suite.Empty(suite.myStore.workspace)
	suite.Empty(suite.myStore.composes)
	suite.Empty(suite.myStore.sources)
	suite.Empty(suite.myStore.blueprintsChanges)
	suite.Empty(suite.myStore.blueprintsCommits)
	suite.Equal(&suite.dir, suite.myStore.stateDir)
}

// Push a blueprint
func (suite *storeTest) TestPushBlueprint() {
	suite.myStore.PushBlueprint(suite.myBP, "testing commit")
	suite.Equal(suite.myBP, suite.myStore.blueprints["testBP"])
	//force a version bump
	suite.myStore.PushBlueprint(suite.myBP, "testing commit")
	suite.Equal("0.0.2", suite.myStore.blueprints["testBP"].Version)
}

// List the blueprint
func (suite *storeTest) TestListBlueprints() {
	suite.myStore.blueprints["testBP"] = suite.myBP
	suite.Equal([]string{"testBP"}, suite.myStore.ListBlueprints())
}

// Push a blueprint to workspace
func (suite *storeTest) TestPushBlueprintToWorkspace() {
	suite.NoError(suite.myStore.PushBlueprintToWorkspace(suite.myBP))
	suite.Equal(suite.myBP, suite.myStore.workspace["testBP"])
}

func (suite *storeTest) TestGetBlueprint() {
	suite.myStore.blueprints["testBP"] = suite.myBP
	suite.myStore.workspace["WIPtestBP"] = suite.myBP
	//Get pushed BP
	actualBP, inWorkspace := suite.myStore.GetBlueprint("testBP")
	suite.Equal(&suite.myBP, actualBP)
	suite.False(inWorkspace)
	//Get BP in worskapce
	actualBP, inWorkspace = suite.myStore.GetBlueprint("WIPtestBP")
	suite.Equal(&suite.myBP, actualBP)
	suite.True(inWorkspace)
	//Try to get a non existing BP
	actualBP, inWorkspace = suite.myStore.GetBlueprint("Non_existing_BP")
	suite.Empty(actualBP)
	suite.False(inWorkspace)
}

func (suite *storeTest) TestGetBlueprintCommited() {
	suite.myStore.blueprints["testBP"] = suite.myBP
	//Get pushed BP
	actualBP := suite.myStore.GetBlueprintCommitted("testBP")
	suite.Equal(&suite.myBP, actualBP)
	//Try to get workspace BP
	actualBP = suite.myStore.GetBlueprintCommitted("WIPtestBP")
	suite.Empty(actualBP)
}

func (suite *storeTest) TestGetBlueprintChanges() {
	suite.myStore.blueprintsCommits["testBP"] = []string{"firstCommit", "secondCommit"}
	actualChanges := suite.myStore.GetBlueprintChanges("testBP")
	suite.Len(actualChanges, 2)
}

func (suite *storeTest) TestGetBlueprintChange() {
	Commit := make(map[string]blueprint.Change)
	Commit[suite.CommitHash[0]] = suite.myChange[0]
	Commit[suite.CommitHash[1]] = suite.myChange[1]
	suite.myStore.blueprintsCommits["testBP"] = suite.CommitHash
	suite.myStore.blueprintsChanges["testBP"] = Commit

	actualChange, err := suite.myStore.GetBlueprintChange("testBP", suite.CommitHash[0])
	suite.NoError(err)
	expectedChange := suite.myChange[0]
	suite.Equal(&expectedChange, actualChange)

	//Try to get non existing BP
	actualChange, err = suite.myStore.GetBlueprintChange("Non_existing_BP", suite.CommitHash[0])
	suite.Nil(actualChange)
	suite.EqualError(err, "Unknown blueprint")

	//Try to get a non existing Commit
	actualChange, err = suite.myStore.GetBlueprintChange("testBP", "Non_existing_commit")
	suite.Nil(actualChange)
	suite.EqualError(err, "Unknown commit")
}

func (suite *storeTest) TestTagBlueprint() {
	Commit := make(map[string]blueprint.Change)
	Commit[suite.CommitHash[0]] = suite.myChange[0]
	Commit[suite.CommitHash[1]] = suite.myChange[1]
	suite.myStore.blueprintsCommits["testBP"] = suite.CommitHash
	suite.myStore.blueprintsChanges["testBP"] = Commit
	suite.myStore.blueprints["testBP"] = suite.myBPv3

	//Check that the blueprint changes have no revisions
	suite.Nil(suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[0]].Revision)
	suite.Nil(suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[1]].Revision)

	// This should tag the most recent commit
	suite.NoError(suite.myStore.TagBlueprint("testBP"))

	actualRevision := suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[0]].Revision
	suite.Nil(actualRevision)

	actualRevision = suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[1]].Revision
	suite.Require().NotNil(actualRevision)
	suite.Equal(1, *actualRevision)

	// Check the blueprints to make sure they have not been changed
	actualBP := suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[0]].Blueprint
	suite.Equal(suite.myBP, actualBP)

	actualBP = suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[1]].Blueprint
	suite.Equal(suite.myBPv3, actualBP)
	suite.Equal(suite.myBPv3, suite.myStore.blueprints["testBP"])

	//Try to tag it again (should not change)
	suite.NoError(suite.myStore.TagBlueprint("testBP"))

	actualRevision = suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[0]].Revision
	suite.Nil(actualRevision)

	actualRevision = suite.myStore.blueprintsChanges["testBP"][suite.CommitHash[1]].Revision
	suite.Require().NotNil(actualRevision)
	suite.Equal(1, *actualRevision)

	//Try to tag a non existing BNP
	suite.EqualError(suite.myStore.TagBlueprint("Non_existing_BP"), "Unknown blueprint")
	//Remove commits from a blueprint and try to tag it
	suite.myStore.blueprintsCommits["testBP"] = []string{}
	suite.EqualError(suite.myStore.TagBlueprint("testBP"), "No commits for blueprint")
}

func (suite *storeTest) TestDeleteBlueprint() {
	suite.myStore.blueprints["testBP"] = suite.myBP
	suite.NoError(suite.myStore.DeleteBlueprint("testBP"))
	suite.Empty(suite.myStore.blueprints)
	//Try to delete again (should return an error)
	suite.EqualError(suite.myStore.DeleteBlueprint("testBP"), "Unknown blueprint: testBP")
}

func (suite *storeTest) TestDeleteBlueprintFromWorkspace() {
	suite.myStore.workspace["WIPtestBP"] = suite.myBP
	suite.NoError(suite.myStore.DeleteBlueprintFromWorkspace("WIPtestBP"))
	suite.Empty(suite.myStore.workspace)
	//Try to delete again (should return an error)
	suite.EqualError(suite.myStore.DeleteBlueprintFromWorkspace("WIPtestBP"), "Unknown blueprint: WIPtestBP")
}

func (suite *storeTest) TestPushCompose() {
	testID := uuid.New()
	err := suite.myStore.PushCompose(testID, suite.myManifest, suite.myImageType, &suite.myBP, 123, nil, uuid.New(), []rpmmd.PackageSpec{})
	suite.NoError(err)
	suite.Panics(func() {
		err = suite.myStore.PushCompose(testID, suite.myManifest, suite.myImageType, &suite.myBP, 123, []*target.Target{suite.myTarget}, uuid.New(), []rpmmd.PackageSpec{})
	})
	suite.NoError(err)

	// Test with PackageSets
	testID = uuid.New()
	err = suite.myStore.PushCompose(testID, suite.myManifest, suite.myImageType, &suite.myBP, 123, nil, uuid.New(), suite.myPackages)
	suite.NoError(err)
}

func (suite *storeTest) TestPushTestCompose() {
	ID := uuid.New()
	err := suite.myStore.PushTestCompose(ID, suite.myManifest, suite.myImageType, &suite.myBP, 123, nil, true, []rpmmd.PackageSpec{})
	suite.NoError(err)
	suite.Equal(common.ImageBuildState(2), suite.myStore.composes[ID].ImageBuild.QueueStatus)
	ID = uuid.New()
	err = suite.myStore.PushTestCompose(ID, suite.myManifest, suite.myImageType, &suite.myBP, 123, []*target.Target{suite.myTarget}, false, []rpmmd.PackageSpec{})
	suite.NoError(err)
	suite.Equal(common.ImageBuildState(3), suite.myStore.composes[ID].ImageBuild.QueueStatus)

	// Test with PackageSets
	ID = uuid.New()
	err = suite.myStore.PushTestCompose(ID, suite.myManifest, suite.myImageType, &suite.myBP, 123, nil, true, suite.myPackages)
	suite.NoError(err)
	suite.Equal(common.ImageBuildState(2), suite.myStore.composes[ID].ImageBuild.QueueStatus)
	ID = uuid.New()
	err = suite.myStore.PushTestCompose(ID, suite.myManifest, suite.myImageType, &suite.myBP, 123, []*target.Target{suite.myTarget}, false, suite.myPackages)
	suite.NoError(err)
	suite.Equal(common.ImageBuildState(3), suite.myStore.composes[ID].ImageBuild.QueueStatus)
}

func (suite *storeTest) TestGetAllComposes() {
	suite.myStore.composes = make(map[uuid.UUID]Compose)
	suite.myStore.composes[uuid.New()] = suite.myCompose
	compose := suite.myStore.GetAllComposes()
	suite.Equal(suite.myStore.composes, compose)
}

func (suite *storeTest) TestDeleteCompose() {
	ID := uuid.New()
	suite.myStore.composes = make(map[uuid.UUID]Compose)
	suite.myStore.composes[ID] = suite.myCompose
	err := suite.myStore.DeleteCompose(ID)
	suite.NoError(err)
	suite.Equal(suite.myStore.composes, map[uuid.UUID]Compose{})
	err = suite.myStore.DeleteCompose(ID)
	suite.Error(err)
}

func (suite *storeTest) TestDeleteSourceByName() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	suite.myStore.DeleteSourceByName("testSourceConfig")
	suite.Equal(map[string]SourceConfig{}, suite.myStore.sources)
}

func (suite *storeTest) TestDeleteSourceByID() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	suite.myStore.DeleteSourceByID("testSource")
	suite.Equal(map[string]SourceConfig{}, suite.myStore.sources)
}

func (suite *storeTest) TestPushSource() {
	expectedSource := map[string]SourceConfig{"testKey": SourceConfig{Name: "testSourceConfig", Type: "", URL: "", CheckGPG: false, CheckSSL: false, System: false}}
	suite.myStore.PushSource("testKey", suite.mySourceConfig)
	suite.Equal(expectedSource, suite.myStore.sources)
}

func (suite *storeTest) TestListSourcesByName() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	actualSources := suite.myStore.ListSourcesByName()
	suite.Equal([]string([]string{"testSourceConfig"}), actualSources)
}

func (suite *storeTest) TestListSourcesById() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	actualSources := suite.myStore.ListSourcesById()
	suite.Equal([]string([]string{"testSource"}), actualSources)
}

func (suite *storeTest) TestGetSource() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	expectedSource := SourceConfig(SourceConfig{Name: "testSourceConfig", Type: "", URL: "", CheckGPG: false, CheckSSL: false, System: false})
	actualSource := suite.myStore.GetSource("testSource")
	suite.Equal(&expectedSource, actualSource)
	actualSource = suite.myStore.GetSource("nonExistingSource")
	suite.Nil(actualSource)
}

func (suite *storeTest) TestGetAllSourcesByName() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	expectedSource := map[string]SourceConfig{"testSourceConfig": SourceConfig{Name: "testSourceConfig", Type: "", URL: "", CheckGPG: false, CheckSSL: false, System: false}}
	actualSource := suite.myStore.GetAllSourcesByName()
	suite.Equal(expectedSource, actualSource)
}

func (suite *storeTest) TestGetAllSourcesByID() {
	suite.myStore.sources = make(map[string]SourceConfig)
	suite.myStore.sources["testSource"] = suite.mySourceConfig
	expectedSource := map[string]SourceConfig{"testSource": SourceConfig{Name: "testSourceConfig", Type: "", URL: "", CheckGPG: false, CheckSSL: false, System: false}}
	actualSource := suite.myStore.GetAllSourcesByID()
	suite.Equal(expectedSource, actualSource)
}

func (suite *storeTest) TestNewSourceConfigWithBaseURL() {
	myRepoConfig := rpmmd.RepoConfig{
		Name:     "testRepo",
		BaseURLs: []string{"testURL"},
		CheckGPG: common.ToPtr(true),
	}
	expectedSource := SourceConfig{Name: "testRepo", Type: "yum-baseurl", URL: "testURL", CheckGPG: true, CheckSSL: true, System: true}
	actualSource := NewSourceConfig(myRepoConfig, true)
	suite.Equal(expectedSource, actualSource)
}

func (suite *storeTest) TestNewSourceConfigWithMetaLink() {
	myRepoConfig := rpmmd.RepoConfig{
		Name:     "testRepo",
		Metalink: "testURL",
		CheckGPG: common.ToPtr(true),
	}
	expectedSource := SourceConfig{Name: "testRepo", Type: "yum-metalink", URL: "testURL", CheckGPG: true, CheckSSL: true, System: true}
	actualSource := NewSourceConfig(myRepoConfig, true)
	suite.Equal(expectedSource, actualSource)
}

func (suite *storeTest) TestNewSourceConfigWithMirrorList() {
	myRepoConfig := rpmmd.RepoConfig{
		Name:       "testRepo",
		MirrorList: "testURL",
	}
	expectedSource := SourceConfig{Name: "testRepo", Type: "yum-mirrorlist", URL: "testURL", CheckGPG: false, CheckSSL: true, System: true}
	actualSource := NewSourceConfig(myRepoConfig, true)
	suite.Equal(expectedSource, actualSource)
}

// Test converting a SourceConfig with GPGkeys to a RepoConfig
func (suite *storeTest) TestRepoConfigGPGKeys() {
	expectedRepo := rpmmd.RepoConfig{Name: "testSourceConfig", BaseURLs: []string{"testURL"}, Metalink: "", MirrorList: "", IgnoreSSL: common.ToPtr(true), MetadataExpire: "", CheckGPG: common.ToPtr(false), CheckRepoGPG: common.ToPtr(true), GPGKeys: []string{"http://path.to.gpgkeys/key.pub", "-----BEGIN PGP PUBLIC KEY BLOCK-----\nFULL GPG KEY HERE\n-----END PGP PUBLIC KEY BLOCK-----"}}
	mySourceConfig := suite.mySourceConfig
	mySourceConfig.Type = "yum-baseurl"
	mySourceConfig.URL = "testURL"
	mySourceConfig.CheckRepoGPG = true
	mySourceConfig.GPGKeys = []string{"http://path.to.gpgkeys/key.pub", "-----BEGIN PGP PUBLIC KEY BLOCK-----\nFULL GPG KEY HERE\n-----END PGP PUBLIC KEY BLOCK-----"}
	actualRepo := mySourceConfig.RepoConfig("testSourceConfig")
	suite.Equal(expectedRepo, actualRepo)
}

func (suite *storeTest) TestRepoConfigBaseURL() {
	expectedRepo := rpmmd.RepoConfig{Name: "testSourceConfig", BaseURLs: []string{"testURL"}, Metalink: "", MirrorList: "", IgnoreSSL: common.ToPtr(true), CheckGPG: common.ToPtr(false), CheckRepoGPG: common.ToPtr(false), MetadataExpire: ""}
	suite.mySourceConfig.Type = "yum-baseurl"
	suite.mySourceConfig.URL = "testURL"
	actualRepo := suite.mySourceConfig.RepoConfig("testSourceConfig")
	suite.Equal(expectedRepo, actualRepo)
}

func (suite *storeTest) TestRepoConfigMetalink() {
	expectedRepo := rpmmd.RepoConfig{Name: "testSourceConfig", Metalink: "testURL", MirrorList: "", IgnoreSSL: common.ToPtr(true), CheckGPG: common.ToPtr(false), CheckRepoGPG: common.ToPtr(false), MetadataExpire: ""}
	suite.mySourceConfig.Type = "yum-metalink"
	suite.mySourceConfig.URL = "testURL"
	actualRepo := suite.mySourceConfig.RepoConfig("testSourceConfig")
	suite.Equal(expectedRepo, actualRepo)
}

func (suite *storeTest) TestRepoConfigMirrorlist() {
	expectedRepo := rpmmd.RepoConfig{Name: "testSourceConfig", Metalink: "", MirrorList: "testURL", IgnoreSSL: common.ToPtr(true), CheckGPG: common.ToPtr(false), CheckRepoGPG: common.ToPtr(false), MetadataExpire: ""}
	suite.mySourceConfig.Type = "yum-mirrorlist"
	suite.mySourceConfig.URL = "testURL"
	actualRepo := suite.mySourceConfig.RepoConfig("testSourceConfig")
	suite.Equal(expectedRepo, actualRepo)
}

// Test multiple SourceConfigs with different CheckGPG and CheckRepoGPG settings
func (suite *storeTest) TestSourceConfigGPGKeysTrueFalse() {
	// We only care about the GPG bools
	sources := []SourceConfig{
		{Name: "source-with-true", CheckGPG: true, CheckRepoGPG: true},
		{Name: "source-with-false", CheckGPG: false, CheckRepoGPG: false},
	}

	// source is reused inside the loop, which can result in unexpected changes in go < 1.22
	// https://go.dev/blog/loopvar-preview
	var repos []rpmmd.RepoConfig
	for _, source := range sources {
		repos = append(repos, source.RepoConfig(source.Name))
	}

	// First repo should be true, second should be false
	suite.True(*repos[0].CheckGPG)
	suite.True(*repos[0].CheckRepoGPG)
	suite.False(*repos[1].CheckGPG)
	suite.False(*repos[1].CheckRepoGPG)

	// We only care about the GPG bools
	// Test with false then true
	sources = []SourceConfig{
		{Name: "source-with-false", CheckGPG: false, CheckRepoGPG: false},
		{Name: "source-with-true", CheckGPG: true, CheckRepoGPG: true},
	}

	repos = []rpmmd.RepoConfig{}
	for _, source := range sources {
		repos = append(repos, source.RepoConfig(source.Name))
	}

	// First repo should be false, second should be true
	suite.False(*repos[0].CheckGPG)
	suite.False(*repos[0].CheckRepoGPG)
	suite.True(*repos[1].CheckGPG)
	suite.True(*repos[1].CheckRepoGPG)
}

func TestStore(t *testing.T) {
	suite.Run(t, new(storeTest))
}
