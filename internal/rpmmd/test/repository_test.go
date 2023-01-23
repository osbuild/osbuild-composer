package rpmmdtests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
)

func getConfPaths(t *testing.T) []string {
	confPaths := []string{
		"./confpaths/priority1",
		"./confpaths/priority2",
	}
	var absConfPaths []string

	for _, path := range confPaths {
		absPath, err := filepath.Abs(path)
		assert.Nil(t, err)
		absConfPaths = append(absConfPaths, absPath)
	}

	return absConfPaths
}

func TestLoadRepositoriesExisting(t *testing.T) {
	confPaths := getConfPaths(t)
	type args struct {
		distro string
	}
	tests := []struct {
		name string
		args args
		want map[string][]string
	}{
		{
			name: "duplicate distro definition, load first encounter",
			args: args{
				distro: test_distro.TestDistroName,
			},
			want: map[string][]string{
				test_distro.TestArchName:  {"fedora-p1", "updates-p1", "fedora-modular-p1", "updates-modular-p1"},
				test_distro.TestArch2Name: {"fedora-p1", "updates-p1", "fedora-modular-p1", "updates-modular-p1"},
			},
		},
		{
			name: "single distro definition",
			args: args{
				distro: test_distro.TestDistro2Name,
			},
			want: map[string][]string{
				test_distro.TestArchName:  {"baseos-p2", "appstream-p2"},
				test_distro.TestArch2Name: {"baseos-p2", "appstream-p2", "google-compute-engine", "google-cloud-sdk"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rpmmd.LoadRepositories(confPaths, tt.args.distro)
			assert.Nil(t, err)

			for wantArch, wantRepos := range tt.want {
				gotArchRepos, exists := got[wantArch]
				assert.True(t, exists, "Expected '%s' arch in repos definition for '%s', but it does not exist", wantArch, tt.args.distro)

				var gotNames []string
				for _, r := range gotArchRepos {
					gotNames = append(gotNames, r.Name)
				}

				if !reflect.DeepEqual(gotNames, wantRepos) {
					t.Errorf("LoadRepositories() for %s/%s =\n got: %#v\n want: %#v", tt.args.distro, wantArch, gotNames, wantRepos)
				}
			}

		})
	}
}

func TestLoadRepositoriesNonExisting(t *testing.T) {
	confPaths := getConfPaths(t)
	repos, err := rpmmd.LoadRepositories(confPaths, "my-imaginary-distro")
	assert.Nil(t, repos)
	assert.NotNil(t, err)
}

func Test_LoadAllRepositories(t *testing.T) {
	confPaths := getConfPaths(t)

	distroReposMap, err := rpmmd.LoadAllRepositories(confPaths)
	assert.NotNil(t, distroReposMap)
	assert.Nil(t, err)
	assert.Equal(t, len(distroReposMap), 2)

	// test-distro
	testDistroRepos, exists := distroReposMap[test_distro.TestDistroName]
	assert.True(t, exists)
	assert.Equal(t, len(testDistroRepos), 2)

	// test-distro - arches
	for _, arch := range []string{test_distro.TestArchName, test_distro.TestArch2Name} {
		testDistroArchRepos, exists := testDistroRepos[arch]
		assert.True(t, exists)
		assert.Equal(t, len(testDistroArchRepos), 4)

		var repoNames []string
		for _, r := range testDistroArchRepos {
			repoNames = append(repoNames, r.Name)
		}

		wantRepos := []string{"fedora-p1", "updates-p1", "fedora-modular-p1", "updates-modular-p1"}

		if !reflect.DeepEqual(repoNames, wantRepos) {
			t.Errorf("LoadAllRepositories() for %s/%s =\n got: %#v\n want: %#v", test_distro.TestDistroName, arch, repoNames, wantRepos)
		}
	}

	// test-distro-2
	testDistro2Repos, exists := distroReposMap[test_distro.TestDistro2Name]
	assert.True(t, exists)
	assert.Equal(t, len(testDistro2Repos), 2)

	// test-distro-2 - arches
	wantRepos := map[string][]string{
		test_distro.TestArchName:  {"baseos-p2", "appstream-p2"},
		test_distro.TestArch2Name: {"baseos-p2", "appstream-p2", "google-compute-engine", "google-cloud-sdk"},
	}
	for _, arch := range []string{test_distro.TestArchName, test_distro.TestArch2Name} {
		testDistro2ArchRepos, exists := testDistro2Repos[arch]
		assert.True(t, exists)
		assert.Equal(t, len(testDistro2ArchRepos), len(wantRepos[arch]))

		var repoNames []string
		for _, r := range testDistro2ArchRepos {
			repoNames = append(repoNames, r.Name)
		}

		if !reflect.DeepEqual(repoNames, wantRepos[arch]) {
			t.Errorf("LoadAllRepositories() for %s/%s =\n got: %#v\n want: %#v", test_distro.TestDistro2Name, arch, repoNames, wantRepos[arch])
		}
	}
}

func TestPackageSetResolveConflictExclude(t *testing.T) {
	tests := []struct {
		got  rpmmd.PackageSet
		want rpmmd.PackageSet
	}{
		{
			got: rpmmd.PackageSet{
				Include: []string{"kernel", "microcode_ctl", "dnf"},
				Exclude: []string{"microcode_ctl"},
			},
			want: rpmmd.PackageSet{
				Include: []string{"kernel", "dnf"},
				Exclude: []string{"microcode_ctl"},
			},
		},
		{
			got: rpmmd.PackageSet{
				Include: []string{"kernel", "dnf"},
				Exclude: []string{"microcode_ctl"},
			},
			want: rpmmd.PackageSet{
				Include: []string{"kernel", "dnf"},
				Exclude: []string{"microcode_ctl"},
			},
		},
		{
			got: rpmmd.PackageSet{
				Include: []string{"kernel", "microcode_ctl", "dnf"},
				Exclude: []string{},
			},
			want: rpmmd.PackageSet{
				Include: []string{"kernel", "microcode_ctl", "dnf"},
				Exclude: []string{},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			if !reflect.DeepEqual(tt.got.ResolveConflictsExclude(), tt.want) {
				t.Errorf("ResolveConflictExclude() returned unexpected result got: %#v\n want: %#v", tt.got.ResolveConflictsExclude(), tt.want)
			}
		})
	}
}

func TestOldWorkerRepositoryCompatUnmarshal(t *testing.T) {
	testCases := []struct {
		repoJSON []byte
		repo     rpmmd.RepoConfig
	}{
		{
			repoJSON: []byte(`{"name":"fedora","baseurl":"http://example.com/fedora"}`),
			repo: rpmmd.RepoConfig{
				Name:     "fedora",
				BaseURLs: []string{"http://example.com/fedora"},
			},
		},
		{
			repoJSON: []byte(`{"name":"multiple","baseurl":"http://example.com/one,http://example.com/two"}`),
			repo: rpmmd.RepoConfig{
				Name:     "multiple",
				BaseURLs: []string{"http://example.com/one", "http://example.com/two"},
			},
		},
		{
			repoJSON: []byte(`{"name":"all","baseurls":["http://example.com/all"],"metalink":"http://example.com/metalink","mirrorlist":"http://example.com/mirrorlist","gpgkeys":["key1","key2"],"check_gpg":true,"check_repo_gpg":true,"ignore_ssl":true,"metadata_expire":"test","rhsm":true,"image_type_tags":["one","two"],"package_sets":["1","2"],"baseurl":"http://example.com/all"}`),
			repo: rpmmd.RepoConfig{
				Name:           "all",
				BaseURLs:       []string{"http://example.com/all"},
				Metalink:       "http://example.com/metalink",
				MirrorList:     "http://example.com/mirrorlist",
				GPGKeys:        []string{"key1", "key2"},
				CheckGPG:       true,
				CheckRepoGPG:   true,
				IgnoreSSL:      true,
				MetadataExpire: "test",
				RHSM:           true,
				ImageTypeTags:  []string{"one", "two"},
				PackageSets:    []string{"1", "2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.repo.Name, func(t *testing.T) {
			var repo rpmmd.RepoConfig
			err := json.Unmarshal(tc.repoJSON, &repo)
			assert.Nil(t, err)
			assert.Equal(t, tc.repo, repo)
		})
	}
}

func TestOldWorkerRepositoryCompatMarshal(t *testing.T) {
	testCases := []struct {
		repoJSON []byte
		repo     rpmmd.RepoConfig
	}{
		{
			repoJSON: []byte(`{"name":"fedora","baseurls":["http://example.com/fedora"],"baseurl":"http://example.com/fedora"}`),
			repo: rpmmd.RepoConfig{
				Name:     "fedora",
				BaseURLs: []string{"http://example.com/fedora"},
			},
		},
		{
			repoJSON: []byte(`{"name":"multiple","baseurls":["http://example.com/one","http://example.com/two"],"baseurl":"http://example.com/one,http://example.com/two"}`),
			repo: rpmmd.RepoConfig{
				Name:     "multiple",
				BaseURLs: []string{"http://example.com/one", "http://example.com/two"},
			},
		},
		{
			repoJSON: []byte(`{"name":"all","baseurls":["http://example.com/all"],"metalink":"http://example.com/metalink","mirrorlist":"http://example.com/mirrorlist","gpgkeys":["key1","key2"],"check_gpg":true,"check_repo_gpg":true,"ignore_ssl":true,"metadata_expire":"test","rhsm":true,"image_type_tags":["one","two"],"package_sets":["1","2"],"baseurl":"http://example.com/all"}`),
			repo: rpmmd.RepoConfig{
				Name:           "all",
				BaseURLs:       []string{"http://example.com/all"},
				Metalink:       "http://example.com/metalink",
				MirrorList:     "http://example.com/mirrorlist",
				GPGKeys:        []string{"key1", "key2"},
				CheckGPG:       true,
				CheckRepoGPG:   true,
				IgnoreSSL:      true,
				MetadataExpire: "test",
				RHSM:           true,
				ImageTypeTags:  []string{"one", "two"},
				PackageSets:    []string{"1", "2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.repo.Name, func(t *testing.T) {
			gotJson, err := json.Marshal(tc.repo)
			assert.Nil(t, err)
			assert.Equal(t, tc.repoJSON, gotJson)
		})
	}
}
