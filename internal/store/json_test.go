package store

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MustParseTime parses a time string and panics if there is an error
func MustParseTime(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	return t
}

func Test_imageTypeToCompatString(t *testing.T) {
	type args struct {
		input distro.ImageType
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				input: testImageType,
			},
			want: test_distro.TestImageTypeName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeToCompatString(tt.args.input)
			if got != tt.want {
				t.Errorf("imageTypeStringToCompatString() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_imageTypeFromCompatString(t *testing.T) {
	type args struct {
		input string
		arch  distro.Arch
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name string
		args args
		want distro.ImageType
	}{
		{
			name: "valid",
			args: args{
				input: test_distro.TestImageTypeName,
				arch:  testArch,
			},
			want: testImageType,
		},
		{
			name: "invalid mapping",
			args: args{
				input: "foo",
				arch:  testArch,
			},
			want: nil,
		},
		{
			name: "invalid distro name",
			args: args{
				input: "test_type_invalid",
				arch:  testArch,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeFromCompatString(tt.args.input, tt.args.arch)
			if got != tt.want {
				t.Errorf("imageTypeStringFromCompatString() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func TestMarshalEmpty(t *testing.T) {
	d := test_distro.New()
	arch, err := d.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", test_distro.TestArchName, err))
	}
	store1 := FixtureEmpty()
	storeV0 := store1.toStoreV0()
	store2 := newStoreFromV0(*storeV0, arch, nil)
	if !reflect.DeepEqual(store1, store2) {
		t.Errorf("marshal/unmarshal roundtrip not a noop for empty store:\n got: %#v\n want: %#v", store1, store2)
	}
}

func TestMarshalFinished(t *testing.T) {
	d := test_distro.New()
	arch, err := d.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(fmt.Sprintf("failed to get architecture %s for a test distro: %v", test_distro.TestArchName, err))
	}
	store1 := FixtureFinished()
	storeV0 := store1.toStoreV0()
	store2 := newStoreFromV0(*storeV0, arch, nil)
	if !reflect.DeepEqual(store1, store2) {
		t.Errorf("marshal/unmarshal roundtrip not a noop for base store:\n got: %#v\n want: %#v", store2, store1)
	}
}

func TestStore_toStoreV0(t *testing.T) {
	type fields struct {
		blueprints        map[string]blueprint.Blueprint
		workspace         map[string]blueprint.Blueprint
		composes          map[uuid.UUID]Compose
		sources           map[string]SourceConfig
		blueprintsChanges map[string]map[string]blueprint.Change
		blueprintsCommits map[string][]string
	}
	tests := []struct {
		name   string
		fields fields
		want   *storeV0
	}{
		{
			name:   "empty",
			fields: fields{},
			want: &storeV0{
				Blueprints: make(blueprintsV0),
				Workspace:  make(workspaceV0),
				Composes:   make(composesV0),
				Sources:    make(sourcesV0),
				Changes:    make(changesV0),
				Commits:    make(commitsV0),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &Store{
				blueprints:        tt.fields.blueprints,
				workspace:         tt.fields.workspace,
				composes:          tt.fields.composes,
				sources:           tt.fields.sources,
				blueprintsChanges: tt.fields.blueprintsChanges,
				blueprintsCommits: tt.fields.blueprintsCommits,
			}
			if got := store.toStoreV0(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Store.toStoreV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newStoreFromV0(t *testing.T) {
	type args struct {
		storeStruct storeV0
		arch        distro.Arch
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)

	tests := []struct {
		name string
		args args
		want *Store
	}{
		{
			name: "empty",
			args: args{
				storeStruct: storeV0{},
				arch:        testArch,
			},
			want: New(nil, testArch, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newStoreFromV0(tt.args.storeStruct, tt.args.arch, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newStoreFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newCommitsV0(t *testing.T) {
	type args struct {
		commits map[string][]string
	}
	tests := []struct {
		name string
		args args
		want commitsV0
	}{
		{
			name: "empty",
			args: args{
				commits: make(map[string][]string),
			},
			want: commitsV0{},
		},
		{
			name: "One blueprint's commits",
			args: args{
				commits: map[string][]string{
					"test-blueprint-changes-v0": {
						"79e2043a83637ffdd4db078c6da23deaae09c84b",
						"72fdb76b9994bd72770e283bf3a5206756daf497",
						"4774980638f4162d9909a613c3ccd938e60bb3a9",
					},
				},
			},
			want: commitsV0{
				"test-blueprint-changes-v0": {
					"79e2043a83637ffdd4db078c6da23deaae09c84b",
					"72fdb76b9994bd72770e283bf3a5206756daf497",
					"4774980638f4162d9909a613c3ccd938e60bb3a9",
				},
			},
		},
		{
			name: "Two blueprint's commits",
			args: args{
				commits: map[string][]string{
					"test-blueprint-changes-v0": {
						"79e2043a83637ffdd4db078c6da23deaae09c84b",
						"72fdb76b9994bd72770e283bf3a5206756daf497",
						"4774980638f4162d9909a613c3ccd938e60bb3a9",
					},
					"second-blueprint": {
						"3c2a2653d044433bae36e3236d394688126fa386",
						"7619ec57c37b4396b5a91358c98792df9e143c18",
						"8d3cc55a6d2841b2bc6e6578d2ec21123110a858",
					},
				},
			},
			want: commitsV0{
				"test-blueprint-changes-v0": {
					"79e2043a83637ffdd4db078c6da23deaae09c84b",
					"72fdb76b9994bd72770e283bf3a5206756daf497",
					"4774980638f4162d9909a613c3ccd938e60bb3a9",
				},
				"second-blueprint": {
					"3c2a2653d044433bae36e3236d394688126fa386",
					"7619ec57c37b4396b5a91358c98792df9e143c18",
					"8d3cc55a6d2841b2bc6e6578d2ec21123110a858",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newCommitsV0(tt.args.commits); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newCommitsV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_upgrade(t *testing.T) {
	assert := assert.New(t)
	testPath, err := filepath.Abs("./test/*.json")
	require.NoError(t, err)
	fileNames, err := filepath.Glob(testPath)
	assert.NoErrorf(err, "Could not read test store directory '%s': %v", testPath, err)
	require.Greaterf(t, len(fileNames), 0, "No test stores found in %s", testPath)
	for _, fileName := range fileNames {
		var storeStruct storeV0
		file, err := ioutil.ReadFile(fileName)
		assert.NoErrorf(err, "Could not read test-store '%s': %v", fileName, err)
		err = json.Unmarshal([]byte(file), &storeStruct)
		assert.NoErrorf(err, "Could not parse test-store '%s': %v", fileName, err)
		arch, err := fedora.NewF35().GetArch(distro.X86_64ArchName)
		assert.NoError(err)
		store := newStoreFromV0(storeStruct, arch, nil)
		assert.Equal(1, len(store.blueprints))
		assert.Equal(1, len(store.blueprintsChanges))
		assert.Equal(1, len(store.blueprintsCommits))
		assert.LessOrEqual(1, len(store.composes))
		assert.Equal(1, len(store.workspace))
	}
}

func Test_newCommitsFromV0(t *testing.T) {
	exampleChanges := changesV0{
		"test-blueprint-changes-v0": {
			"4774980638f4162d9909a613c3ccd938e60bb3a9": {
				Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
				Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
				Revision:  nil,
				Timestamp: "2020-07-29T09:52:07Z",
			},
			"72fdb76b9994bd72770e283bf3a5206756daf497": {
				Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
				Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
				Revision:  nil,
				Timestamp: "2020-07-09T13:33:06Z",
			},
			"79e2043a83637ffdd4db078c6da23deaae09c84b": {
				Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
				Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
				Revision:  nil,
				Timestamp: "2020-07-07T02:57:00Z",
			},
		},
	}
	type args struct {
		changes changesV0
		commits commitsV0
	}
	tests := []struct {
		name string
		args args
		want map[string][]string
	}{
		{
			name: "empty",
			args: args{
				changes: make(changesV0),
				commits: make(commitsV0),
			},
			want: make(map[string][]string),
		},
		{
			name: "Changes with no commits",
			args: args{
				changes: exampleChanges,
				commits: make(commitsV0),
			},
			want: map[string][]string{
				"test-blueprint-changes-v0": {
					// Oldest sorted first
					"79e2043a83637ffdd4db078c6da23deaae09c84b",
					"72fdb76b9994bd72770e283bf3a5206756daf497",
					"4774980638f4162d9909a613c3ccd938e60bb3a9",
				},
			},
		},
		{
			name: "Changes with commits",
			args: args{
				changes: exampleChanges,
				commits: commitsV0{
					"test-blueprint-changes-v0": {
						"79e2043a83637ffdd4db078c6da23deaae09c84b",
						"72fdb76b9994bd72770e283bf3a5206756daf497",
						"4774980638f4162d9909a613c3ccd938e60bb3a9",
					},
				},
			},
			want: map[string][]string{
				"test-blueprint-changes-v0": {
					// Oldest sorted first
					"79e2043a83637ffdd4db078c6da23deaae09c84b",
					"72fdb76b9994bd72770e283bf3a5206756daf497",
					"4774980638f4162d9909a613c3ccd938e60bb3a9",
				},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newCommitsFromV0(tt.args.commits, tt.args.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newCommitsFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newBlueprintsFromV0(t *testing.T) {
	tests := []struct {
		name       string
		blueprints blueprintsV0
		want       map[string]blueprint.Blueprint
	}{
		{
			name:       "empty",
			blueprints: blueprintsV0{},
			want:       make(map[string]blueprint.Blueprint),
		},
		{
			name: "Two Blueprints",
			blueprints: blueprintsV0{
				"blueprint-1": {
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1",
				},
				"blueprint-2": {
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1",
				},
			},
			want: map[string]blueprint.Blueprint{
				"blueprint-1": blueprint.Blueprint{
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1"},
				"blueprint-2": blueprint.Blueprint{
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newBlueprintsFromV0(tt.blueprints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newBlueprintsFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newBlueprintsV0(t *testing.T) {
	tests := []struct {
		name       string
		blueprints map[string]blueprint.Blueprint
		want       blueprintsV0
	}{
		{
			name:       "empty",
			blueprints: make(map[string]blueprint.Blueprint),
			want:       blueprintsV0{},
		},
		{
			name: "Two Blueprints",
			blueprints: map[string]blueprint.Blueprint{
				"blueprint-1": blueprint.Blueprint{
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1"},
				"blueprint-2": blueprint.Blueprint{
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1"},
			},
			want: blueprintsV0{
				"blueprint-1": {
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1",
				},
				"blueprint-2": {
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newBlueprintsV0(tt.blueprints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newBlueprintsV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newWorkspaceFromV0(t *testing.T) {
	tests := []struct {
		name       string
		blueprints workspaceV0
		want       map[string]blueprint.Blueprint
	}{
		{
			name:       "empty",
			blueprints: workspaceV0{},
			want:       make(map[string]blueprint.Blueprint),
		},
		{
			name: "Two Blueprints",
			blueprints: workspaceV0{
				"blueprint-1": {
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1",
				},
				"blueprint-2": {
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1",
				},
			},
			want: map[string]blueprint.Blueprint{
				"blueprint-1": blueprint.Blueprint{
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1"},
				"blueprint-2": blueprint.Blueprint{
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newWorkspaceFromV0(tt.blueprints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newWorkspaceFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newWorkspaceV0(t *testing.T) {
	tests := []struct {
		name       string
		blueprints map[string]blueprint.Blueprint
		want       workspaceV0
	}{
		{
			name:       "empty",
			blueprints: make(map[string]blueprint.Blueprint),
			want:       workspaceV0{},
		},
		{
			name: "Two Blueprints",
			blueprints: map[string]blueprint.Blueprint{
				"blueprint-1": blueprint.Blueprint{
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1"},
				"blueprint-2": blueprint.Blueprint{
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1"},
			},
			want: workspaceV0{
				"blueprint-1": {
					Name:        "blueprint-1",
					Description: "First Blueprint in Test",
					Version:     "0.0.1",
				},
				"blueprint-2": {
					Name:        "blueprint-2",
					Description: "Second Blueprint in Test",
					Version:     "0.0.1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newWorkspaceV0(tt.blueprints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newWorkspaceV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newChangesFromV0(t *testing.T) {
	tests := []struct {
		name    string
		changes changesV0
		want    map[string]map[string]blueprint.Change
	}{
		{
			name:    "empty",
			changes: changesV0{},
			want:    make(map[string]map[string]blueprint.Change),
		},
		{
			name: "One Blueprint's Changes",
			changes: changesV0{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
			want: map[string]map[string]blueprint.Change{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
		},
		{
			name: "Two Blueprint's Changes",
			changes: changesV0{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
				},
				"test-blueprint-changes-2": {
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
			want: map[string]map[string]blueprint.Change{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
				},
				"test-blueprint-changes-2": {
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newChangesFromV0(tt.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newChangesFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newChangesV0(t *testing.T) {
	tests := []struct {
		name    string
		changes map[string]map[string]blueprint.Change
		want    changesV0
	}{
		{
			name:    "empty",
			changes: make(map[string]map[string]blueprint.Change),
			want:    changesV0{},
		},
		{
			name: "One Blueprint's Changes",
			changes: map[string]map[string]blueprint.Change{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
			want: changesV0{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
		},
		{
			name: "Two Blueprint's Changes",
			changes: map[string]map[string]blueprint.Change{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
				},
				"test-blueprint-changes-2": {
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
			want: changesV0{
				"test-blueprint-changes-1": {
					"4774980638f4162d9909a613c3ccd938e60bb3a9": {
						Commit:    "4774980638f4162d9909a613c3ccd938e60bb3a9",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.2 saved.",
						Revision:  nil,
						Timestamp: "2020-07-29T09:52:07Z",
					},
				},
				"test-blueprint-changes-2": {
					"72fdb76b9994bd72770e283bf3a5206756daf497": {
						Commit:    "72fdb76b9994bd72770e283bf3a5206756daf497",
						Message:   "Recipe test-blueprint-changes-v0, version 0.1.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-09T13:33:06Z",
					},
					"79e2043a83637ffdd4db078c6da23deaae09c84b": {
						Commit:    "79e2043a83637ffdd4db078c6da23deaae09c84b",
						Message:   "Recipe test-blueprint-changes-v0, version 0.0.1 saved.",
						Revision:  nil,
						Timestamp: "2020-07-07T02:57:00Z",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newChangesV0(tt.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newChangesV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newSourceConfigsFromV0(t *testing.T) {
	tests := []struct {
		name    string
		sources sourcesV0
		want    map[string]SourceConfig
	}{
		{
			name:    "empty",
			sources: sourcesV0{},
			want:    make(map[string]SourceConfig),
		},
		{
			name: "One Source",
			sources: sourcesV0{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
			want: map[string]SourceConfig{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
		},
		{
			name: "Two Sources",
			sources: sourcesV0{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
				"repo-2": {
					Name:     "testRepo2",
					Type:     "yum-baseurl",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
			want: map[string]SourceConfig{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
				"repo-2": {
					Name:     "testRepo2",
					Type:     "yum-baseurl",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newSourceConfigsFromV0(tt.sources); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newSourceConfigsFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newSourcesFromV0(t *testing.T) {
	tests := []struct {
		name    string
		sources map[string]SourceConfig
		want    sourcesV0
	}{
		{
			name:    "empty",
			sources: make(map[string]SourceConfig),
			want:    sourcesV0{},
		},
		{
			name: "One Source",
			sources: map[string]SourceConfig{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
			want: sourcesV0{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
		},
		{
			name: "Two Sources",
			sources: map[string]SourceConfig{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
				"repo-2": {
					Name:     "testRepo2",
					Type:     "yum-baseurl",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
			want: sourcesV0{
				"repo-1": {
					Name:     "testRepo1",
					Type:     "yum-mirrorlist",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
				"repo-2": {
					Name:     "testRepo2",
					Type:     "yum-baseurl",
					URL:      "testURL",
					CheckGPG: true,
					CheckSSL: true,
					System:   false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newSourcesV0(tt.sources); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newSourcesV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newComposeV0(t *testing.T) {
	bp := blueprint.Blueprint{
		Name:        "tmux",
		Description: "tmux blueprint",
		Version:     "0.0.1",
		Packages: []blueprint.Package{
			{Name: "tmux", Version: "*"}},
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name    string
		compose Compose
		want    composeV0
	}{
		{
			name: "qcow2 compose",
			compose: Compose{
				Blueprint: &bp,
				ImageBuild: ImageBuild{
					ID:        0,
					ImageType: testImageType,
					Manifest:  []byte("JSON MANIFEST GOES HERE"),
					Targets: []*target.Target{
						{
							Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
							ImageName: "",
							Name:      target.TargetNameAWS,
							Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
							Status:    common.IBWaiting,
							OsbuildArtifact: target.OsbuildArtifact{
								ExportFilename: "disk.qcow2",
							},
							Options: target.AWSTargetOptions{
								Region: "us-east-1",
								Bucket: "bucket",
								Key:    "key",
							},
						},
					},
					JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
					JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
					JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
					Size:        2147483648,
					JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
					QueueStatus: common.IBFinished,
				},
			},
			want: composeV0{
				Blueprint: &bp,
				ImageBuilds: []imageBuildV0{
					{
						ID:        0,
						ImageType: "test_type",
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
						QueueStatus: common.IBFinished,
					},
				},
				Packages: []rpmmd.PackageSpec{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newComposeV0(tt.compose); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newComposeV0() =\n got %#v\n want %#v", got, tt.want)
			}
		})
	}
}

func Test_newComposeFromV0(t *testing.T) {
	bp := blueprint.Blueprint{
		Name:        "tmux",
		Description: "tmux blueprint",
		Version:     "0.0.1",
		Packages: []blueprint.Package{
			{Name: "tmux", Version: "*"}},
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name    string
		compose composeV0
		arch    distro.Arch
		want    Compose
		errOk   bool
	}{
		{
			name:    "empty",
			compose: composeV0{},
			arch:    testArch,
			want:    Compose{},
			errOk:   true,
		},
		{
			name:  "qcow2 compose",
			arch:  testArch,
			errOk: false,
			compose: composeV0{
				Blueprint: &bp,
				ImageBuilds: []imageBuildV0{
					{
						ID:        0,
						ImageType: test_distro.TestImageTypeName,
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
						QueueStatus: common.IBFinished,
					},
				},
			},
			want: Compose{
				Blueprint: &bp,
				ImageBuild: ImageBuild{
					ID:        0,
					ImageType: testImageType,
					Manifest:  []byte("JSON MANIFEST GOES HERE"),
					Targets: []*target.Target{
						{
							Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
							ImageName: "",
							Name:      target.TargetNameAWS,
							Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
							Status:    common.IBWaiting,
							OsbuildArtifact: target.OsbuildArtifact{
								ExportFilename: "disk.qcow2",
							},
							Options: target.AWSTargetOptions{
								Region: "us-east-1",
								Bucket: "bucket",
								Key:    "key",
							},
						},
					},
					JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
					JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
					JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
					Size:        2147483648,
					JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
					QueueStatus: common.IBFinished,
				},
				Packages: []rpmmd.PackageSpec{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newComposeFromV0(tt.compose, tt.arch)
			if err != nil {
				if !tt.errOk {
					t.Errorf("newComposeFromV0() error = %v", err)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newComposeFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newComposesV0(t *testing.T) {
	bp := blueprint.Blueprint{
		Name:        "tmux",
		Description: "tmux blueprint",
		Version:     "0.0.1",
		Packages: []blueprint.Package{
			{Name: "tmux", Version: "*"}},
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name     string
		composes map[uuid.UUID]Compose
		want     composesV0
	}{
		{
			name: "two composes",
			composes: map[uuid.UUID]Compose{
				uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"): {
					Blueprint: &bp,
					ImageBuild: ImageBuild{
						ID:        0,
						ImageType: testImageType,
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
						QueueStatus: common.IBFinished,
					},
				},
				uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"): {
					Blueprint: &bp,
					ImageBuild: ImageBuild{
						ID:        0,
						ImageType: testImageType,
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("6ac04049-341a-4297-b50b-5424bec9f193"),
						QueueStatus: common.IBFinished,
					},
				},
			},
			want: composesV0{
				uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"): {
					Blueprint: &bp,
					ImageBuilds: []imageBuildV0{
						{
							ID:        0,
							ImageType: test_distro.TestImageTypeName,
							Manifest:  []byte("JSON MANIFEST GOES HERE"),
							Targets: []*target.Target{
								{
									Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
									ImageName: "",
									Name:      target.TargetNameAWS,
									Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
									Status:    common.IBWaiting,
									OsbuildArtifact: target.OsbuildArtifact{
										ExportFilename: "disk.qcow2",
									},
									Options: target.AWSTargetOptions{
										Region: "us-east-1",
										Bucket: "bucket",
										Key:    "key",
									},
								},
							},
							JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
							JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
							JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
							Size:        2147483648,
							JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
							QueueStatus: common.IBFinished,
						},
					},
					Packages: []rpmmd.PackageSpec{},
				},
				uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"): {
					Blueprint: &bp,
					ImageBuilds: []imageBuildV0{
						{
							ID:        0,
							ImageType: test_distro.TestImageTypeName,
							Manifest:  []byte("JSON MANIFEST GOES HERE"),
							Targets: []*target.Target{
								{
									Uuid:      uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"),
									ImageName: "",
									Name:      target.TargetNameAWS,
									Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
									Status:    common.IBWaiting,
									OsbuildArtifact: target.OsbuildArtifact{
										ExportFilename: "disk.qcow2",
									},
									Options: target.AWSTargetOptions{
										Region: "us-east-1",
										Bucket: "bucket",
										Key:    "key",
									},
								},
							},
							JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
							JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
							JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
							Size:        2147483648,
							JobID:       uuid.MustParse("6ac04049-341a-4297-b50b-5424bec9f193"),
							QueueStatus: common.IBFinished,
						},
					},
					Packages: []rpmmd.PackageSpec{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newComposesV0(tt.composes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newComposesV0() =\n got %#v\n want %#v", got, tt.want)
			}
		})
	}
}

func Test_newComposesFromV0(t *testing.T) {
	bp := blueprint.Blueprint{
		Name:        "tmux",
		Description: "tmux blueprint",
		Version:     "0.0.1",
		Packages: []blueprint.Package{
			{Name: "tmux", Version: "*"}},
	}

	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name     string
		arch     distro.Arch
		composes composesV0
		want     map[uuid.UUID]Compose
	}{
		{
			name:     "empty",
			arch:     testArch,
			composes: composesV0{},
			want:     make(map[uuid.UUID]Compose),
		},
		{
			name: "two composes",
			arch: testArch,
			composes: composesV0{
				uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"): {
					Blueprint: &bp,
					ImageBuilds: []imageBuildV0{
						{
							ID:        0,
							ImageType: "test_type",
							Manifest:  []byte("JSON MANIFEST GOES HERE"),
							Targets: []*target.Target{
								{
									Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
									ImageName: "",
									Name:      target.TargetNameAWS,
									Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
									Status:    common.IBWaiting,
									OsbuildArtifact: target.OsbuildArtifact{
										ExportFilename: "disk.qcow2",
									},
									Options: target.AWSTargetOptions{
										Region: "us-east-1",
										Bucket: "bucket",
										Key:    "key",
									},
								},
							},
							JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
							JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
							JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
							Size:        2147483648,
							JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
							QueueStatus: common.IBFinished,
						},
					},
				},
				uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"): {
					Blueprint: &bp,
					ImageBuilds: []imageBuildV0{
						{
							ID:        0,
							ImageType: "test_type",
							Manifest:  []byte("JSON MANIFEST GOES HERE"),
							Targets: []*target.Target{
								{
									Uuid:      uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"),
									ImageName: "",
									Name:      target.TargetNameAWS,
									Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
									Status:    common.IBWaiting,
									OsbuildArtifact: target.OsbuildArtifact{
										ExportFilename: "disk.qcow2",
									},
									Options: target.AWSTargetOptions{
										Region: "us-east-1",
										Bucket: "bucket",
										Key:    "key",
									},
								},
							},
							JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
							JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
							JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
							Size:        2147483648,
							JobID:       uuid.MustParse("6ac04049-341a-4297-b50b-5424bec9f193"),
							QueueStatus: common.IBFinished,
						},
					},
				},
			},
			want: map[uuid.UUID]Compose{
				uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"): {
					Blueprint: &bp,
					ImageBuild: ImageBuild{
						ID:        0,
						ImageType: testImageType,
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
						QueueStatus: common.IBFinished,
					},
					Packages: []rpmmd.PackageSpec{},
				},
				uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"): {
					Blueprint: &bp,
					ImageBuild: ImageBuild{
						ID:        0,
						ImageType: testImageType,
						Manifest:  []byte("JSON MANIFEST GOES HERE"),
						Targets: []*target.Target{
							{
								Uuid:      uuid.MustParse("14c454d0-26f3-4a56-8ceb-a5673aaba686"),
								ImageName: "",
								Name:      target.TargetNameAWS,
								Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
								Status:    common.IBWaiting,
								OsbuildArtifact: target.OsbuildArtifact{
									ExportFilename: "disk.qcow2",
								},
								Options: target.AWSTargetOptions{
									Region: "us-east-1",
									Bucket: "bucket",
									Key:    "key",
								},
							},
						},
						JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
						JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
						JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
						Size:        2147483648,
						JobID:       uuid.MustParse("6ac04049-341a-4297-b50b-5424bec9f193"),
						QueueStatus: common.IBFinished,
					},
					Packages: []rpmmd.PackageSpec{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newComposesFromV0(tt.composes, tt.arch, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newComposesFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}

func Test_newImageBuildFromV0(t *testing.T) {
	testDistro := test_distro.New()
	testArch, _ := testDistro.GetArch(test_distro.TestArchName)
	testImageType, _ := testArch.GetImageType(test_distro.TestImageTypeName)

	tests := []struct {
		name  string
		arch  distro.Arch
		ib    imageBuildV0
		want  ImageBuild
		errOk bool
	}{
		{
			name:  "empty",
			arch:  testArch,
			errOk: true,
			ib:    imageBuildV0{},
			want:  ImageBuild{},
		},
		{
			name:  "qcow2 image build",
			arch:  testArch,
			errOk: false,
			ib: imageBuildV0{
				ID:        0,
				ImageType: test_distro.TestImageTypeName,
				Manifest:  []byte("JSON MANIFEST GOES HERE"),
				Targets: []*target.Target{
					{
						Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
						ImageName: "",
						Name:      target.TargetNameAWS,
						Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
						Status:    common.IBWaiting,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportFilename: "disk.qcow2",
						},
						Options: target.AWSTargetOptions{
							Region: "us-east-1",
							Bucket: "bucket",
							Key:    "key",
						},
					},
				},
				JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
				JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
				JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
				Size:        2147483648,
				JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
				QueueStatus: common.IBFinished,
			},
			want: ImageBuild{
				ID:        0,
				ImageType: testImageType,
				Manifest:  []byte("JSON MANIFEST GOES HERE"),
				Targets: []*target.Target{
					{
						Uuid:      uuid.MustParse("f53b49c0-d321-447e-8ab8-6e827891e3f0"),
						ImageName: "",
						Name:      target.TargetNameAWS,
						Created:   MustParseTime("2020-08-12T09:21:44.427717205-07:00"),
						Status:    common.IBWaiting,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportFilename: "disk.qcow2",
						},
						Options: target.AWSTargetOptions{
							Region: "us-east-1",
							Bucket: "bucket",
							Key:    "key",
						},
					},
				},
				JobCreated:  MustParseTime("2020-08-12T09:21:50.07040195-07:00"),
				JobStarted:  MustParseTime("0001-01-01T00:00:00Z"),
				JobFinished: MustParseTime("0001-01-01T00:00:00Z"),
				Size:        2147483648,
				JobID:       uuid.MustParse("22445cd3-7fa5-4dca-b7f8-4f9857b3e3a0"),
				QueueStatus: common.IBFinished,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newImageBuildFromV0(tt.ib, tt.arch)
			if err != nil {
				if !tt.errOk {
					t.Errorf("newImageBuildFromV0() error = %v", err)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newImageBuildFromV0() =\n got: %#v\n want: %#v", got, tt.want)
			}
		})
	}
}
