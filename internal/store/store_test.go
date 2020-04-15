package store

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"os"
	"testing"
)

//struct for sharing state between tests
type storeTest struct {
	suite.Suite
	dir              string
	myStore          *Store
	myCustomizations blueprint.Customizations
	myBP             blueprint.Blueprint
	CommitHash       string
	myChange         blueprint.Change
}

//func to initialize some default values before the suite is ran
func (suite *storeTest) SetupSuite() {
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
		Customizations: &suite.myCustomizations,
	}
	suite.CommitHash = "firstCommit"
	suite.myChange = blueprint.Change{
		Commit:    "firstCommit",
		Message:   "firstCommitMessage",
		Revision:  nil,
		Timestamp: "now",
		Blueprint: suite.myBP,
	}
}

//setup before each test
func (suite *storeTest) SetupTest() {
	tmpDir, err := ioutil.TempDir("/tmp", "osbuild-composer-test-")
	suite.NoError(err)
	suite.dir = tmpDir
	suite.myStore = New(&suite.dir)
}

//teardown after each test
func (suite *storeTest) TearDownTest() {
	os.RemoveAll(suite.dir)
}

func (suite *storeTest) TestRandomSHA1String() {
	hash, err := randomSHA1String()
	suite.NoError(err)
	suite.Len(hash, 40)
}

//Check initial state of fields
func (suite *storeTest) TestNewEmpty() {
	suite.Empty(suite.myStore.Blueprints)
	suite.Empty(suite.myStore.Workspace)
	suite.Empty(suite.myStore.Composes)
	suite.Empty(suite.myStore.Sources)
	suite.Empty(suite.myStore.BlueprintsChanges)
	suite.Empty(suite.myStore.BlueprintsCommits)
	suite.Empty(suite.myStore.pendingJobs)
	suite.Equal(&suite.dir, suite.myStore.stateDir)
}

//Push a blueprint
func (suite *storeTest) TestPushBlueprint() {
	suite.myStore.PushBlueprint(suite.myBP, "testing commit")
	suite.Equal(suite.myBP, suite.myStore.Blueprints["testBP"])
	//force a version bump
	suite.myStore.PushBlueprint(suite.myBP, "testing commit")
	suite.Equal("0.0.2", suite.myStore.Blueprints["testBP"].Version)
}

//List the blueprint
func (suite *storeTest) TestListBlueprints() {
	suite.myStore.Blueprints["testBP"] = suite.myBP
	suite.Equal([]string{"testBP"}, suite.myStore.ListBlueprints())
}

//Push a blueprint to workspace
func (suite *storeTest) TestPushBlueprintToWorkspace() {
	suite.NoError(suite.myStore.PushBlueprintToWorkspace(suite.myBP))
	suite.Equal(suite.myBP, suite.myStore.Workspace["testBP"])
}

func (suite *storeTest) TestGetBlueprint() {
	suite.myStore.Blueprints["testBP"] = suite.myBP
	suite.myStore.Workspace["WIPtestBP"] = suite.myBP
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
	suite.myStore.Blueprints["testBP"] = suite.myBP
	//Get pushed BP
	actualBP := suite.myStore.GetBlueprintCommitted("testBP")
	suite.Equal(&suite.myBP, actualBP)
	//Try to get workspace BP
	actualBP = suite.myStore.GetBlueprintCommitted("WIPtestBP")
	suite.Empty(actualBP)
}

func (suite *storeTest) TestGetBlueprintChanges() {
	suite.myStore.BlueprintsCommits["testBP"] = []string{"firstCommit", "secondCommit"}
	actualChanges := suite.myStore.GetBlueprintChanges("testBP")
	suite.Len(actualChanges, 2)
}

func (suite *storeTest) TestGetBlueprintChange() {
	Commit := make(map[string]blueprint.Change)
	Commit[suite.CommitHash] = suite.myChange
	suite.myStore.BlueprintsCommits["testBP"] = []string{suite.CommitHash}
	suite.myStore.BlueprintsChanges["testBP"] = Commit

	actualChange, err := suite.myStore.GetBlueprintChange("testBP", suite.CommitHash)
	suite.NoError(err)
	expectedChange := suite.myChange
	suite.Equal(&expectedChange, actualChange)

	//Try to get non existing BP
	actualChange, err = suite.myStore.GetBlueprintChange("Non_existing_BP", suite.CommitHash)
	suite.Nil(actualChange)
	suite.EqualError(err, "Unknown blueprint")

	//Try to get a non existing Commit
	actualChange, err = suite.myStore.GetBlueprintChange("testBP", "Non_existing_commit")
	suite.Nil(actualChange)
	suite.EqualError(err, "Unknown commit")
}

func (suite *storeTest) TestTagBlueprint() {
	Commit := make(map[string]blueprint.Change)
	Commit[suite.CommitHash] = suite.myChange
	suite.myStore.Blueprints["testBP"] = suite.myBP
	suite.myStore.BlueprintsCommits["testBP"] = []string{suite.CommitHash}
	suite.myStore.BlueprintsChanges["testBP"] = Commit

	//Check that the blueprints change has no revision
	suite.Nil(suite.myStore.BlueprintsChanges["testBP"][suite.CommitHash].Revision)
	suite.NoError(suite.myStore.TagBlueprint("testBP"))
	//The blueprints change should have a revision now
	actualRevision := suite.myStore.BlueprintsChanges["testBP"][suite.CommitHash].Revision
	suite.Equal(1, *actualRevision)
	//Try to tag it again (should not change)
	suite.NoError(suite.myStore.TagBlueprint("testBP"))
	suite.Equal(1, *actualRevision)
	//Try to tag a non existing BNP
	suite.EqualError(suite.myStore.TagBlueprint("Non_existing_BP"), "Unknown blueprint")
	//Remove commits from a blueprint and try to tag it
	suite.myStore.BlueprintsCommits["testBP"] = []string{}
	suite.EqualError(suite.myStore.TagBlueprint("testBP"), "No commits for blueprint")
}

func (suite *storeTest) TestDeleteBlueprint() {
	suite.myStore.Blueprints["testBP"] = suite.myBP
	suite.NoError(suite.myStore.DeleteBlueprint("testBP"))
	suite.Empty(suite.myStore.Blueprints)
	//Try to delete again (should return an error)
	suite.EqualError(suite.myStore.DeleteBlueprint("testBP"), "Unknown blueprint: testBP")
}

func (suite *storeTest) TestDeleteBlueprintFromWorkspace() {
	suite.myStore.Workspace["WIPtestBP"] = suite.myBP
	suite.NoError(suite.myStore.DeleteBlueprintFromWorkspace("WIPtestBP"))
	suite.Empty(suite.myStore.Workspace)
	//Try to delete again (should return an error)
	suite.EqualError(suite.myStore.DeleteBlueprintFromWorkspace("WIPtestBP"), "Unknown blueprint: WIPtestBP")
}

func TestStore(t *testing.T) {
	suite.Run(t, new(storeTest))
}
