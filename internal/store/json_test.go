package store

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_imageTypeToCompatString(t *testing.T) {
	type args struct {
		input distro.ImageType
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				input: &test_distro.TestImageType{},
			},
			want: "test_type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeToCompatString(tt.args.input)
			if got != tt.want {
				t.Errorf("imageTypeStringToCompatString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_imageTypeFromCompatString(t *testing.T) {
	type args struct {
		input string
		arch  distro.Arch
	}
	tests := []struct {
		name string
		args args
		want distro.ImageType
	}{
		{
			name: "valid",
			args: args{
				input: "test_type",
				arch:  &test_distro.TestArch{},
			},
			want: &test_distro.TestImageType{},
		},
		{
			name: "invalid mapping",
			args: args{
				input: "foo",
				arch:  &test_distro.TestArch{},
			},
			want: nil,
		},
		{
			name: "invalid distro name",
			args: args{
				input: "test_type_invalid",
				arch:  &test_distro.TestArch{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeFromCompatString(tt.args.input, tt.args.arch)
			if got != tt.want {
				t.Errorf("imageTypeStringFromCompatString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalEmpty(t *testing.T) {
	d := fedoratest.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	store1 := FixtureEmpty()
	storeV0 := store1.toStoreV0()
	store2 := newStoreFromV0(*storeV0, arch, nil)
	if !reflect.DeepEqual(store1, store2) {
		t.Errorf("marshal/unmarshal roundtrip not a noop for empty store: %v != %v", store1, store2)
	}
}

func TestMarshalFinished(t *testing.T) {
	d := fedoratest.New()
	arch, err := d.GetArch("x86_64")
	if err != nil {
		panic("invalid architecture x86_64 for fedoratest")
	}
	store1 := FixtureFinished()
	storeV0 := store1.toStoreV0()
	store2 := newStoreFromV0(*storeV0, arch, nil)
	if !reflect.DeepEqual(store1, store2) {
		t.Errorf("marshal/unmarshal roundtrip not a noop for base store: %v != %v", store1, store2)
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
				t.Errorf("Store.toStoreV0() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newStoreFromV0(t *testing.T) {
	type args struct {
		storeStruct storeV0
		arch        distro.Arch
	}
	tests := []struct {
		name string
		args args
		want *Store
	}{
		{
			name: "empty",
			args: args{
				storeStruct: storeV0{},
				arch:        &test_distro.TestArch{},
			},
			want: New(nil, &test_distro.TestArch{}, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newStoreFromV0(tt.args.storeStruct, tt.args.arch, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newStoreFromV0() = %v, want %v", got, tt.want)
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
				t.Errorf("newCommitsV0() = %v, want %v", got, tt.want)
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
		arch, err := fedora32.New().GetArch("x86_64")
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
				t.Errorf("newCommitsFromV0() = %v, want %v", got, tt.want)
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
				t.Errorf("newBlueprintsFromV0() = %v, want %v", got, tt.want)
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
				t.Errorf("newBlueprintsV0() = %v, want %v", got, tt.want)
			}
		})
	}
}
