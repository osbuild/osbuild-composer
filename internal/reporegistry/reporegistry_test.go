package reporegistry

import (
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
)

func getTestingRepoRegistry() *RepoRegistry {
	return &RepoRegistry{
		map[string]map[string][]rpmmd.RepoConfig{
			test_distro.TestDistroName: {
				test_distro.TestArchName: {
					{
						Name:     "baseos",
						BaseURLs: []string{"https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os"},
					},
					{
						Name:     "appstream",
						BaseURLs: []string{"https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os"},
					},
				},
				test_distro.TestArch2Name: {
					{
						Name:     "baseos",
						BaseURLs: []string{"https://cdn.redhat.com/content/dist/rhel8/8/aarch64/baseos/os"},
					},
					{
						Name:          "appstream",
						BaseURLs:      []string{"https://cdn.redhat.com/content/dist/rhel8/8/aarch64/appstream/os"},
						ImageTypeTags: []string{},
					},
					{
						Name:          "google-compute-engine",
						BaseURLs:      []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"},
						ImageTypeTags: []string{test_distro.TestImageType2Name},
					},
					{
						Name:          "google-cloud-sdk",
						BaseURLs:      []string{"https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64"},
						ImageTypeTags: []string{test_distro.TestImageType2Name},
					},
				},
			},
		},
	}
}

func TestReposByImageType_reposByImageTypeName(t *testing.T) {
	rr := getTestingRepoRegistry()
	testDistro := test_distro.New()

	ta, _ := testDistro.GetArch(test_distro.TestArchName)
	ta2, _ := testDistro.GetArch(test_distro.TestArch2Name)

	ta_it, _ := ta.GetImageType(test_distro.TestImageTypeName)

	ta2_it, _ := ta2.GetImageType(test_distro.TestImageTypeName)
	ta2_it2, _ := ta2.GetImageType(test_distro.TestImageType2Name)

	type args struct {
		input distro.ImageType
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "NoAdditionalReposNeeded_NoAdditionalReposDefined",
			args: args{
				input: ta_it,
			},
			want: []string{"baseos", "appstream"},
		},
		{
			name: "NoAdditionalReposNeeded_AdditionalReposDefined",
			args: args{
				input: ta2_it,
			},
			want: []string{"baseos", "appstream"},
		},
		{
			name: "AdditionalReposNeeded_AdditionalReposDefined",
			args: args{
				input: ta2_it2,
			},
			want: []string{"baseos", "appstream", "google-compute-engine", "google-cloud-sdk"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rr.ReposByImageType(tt.args.input)
			assert.Nil(t, err)

			var gotNames []string
			for _, r := range got {
				gotNames = append(gotNames, r.Name)
			}

			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Errorf("ReposByImageType() =\n got: %#v\n want: %#v", gotNames, tt.want)
			}

			got, err = rr.reposByImageTypeName(tt.args.input.Arch().Distro().Name(), tt.args.input.Arch().Name(), tt.args.input.Name())
			assert.Nil(t, err)
			gotNames = []string{}
			for _, r := range got {
				gotNames = append(gotNames, r.Name)
			}

			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Errorf("reposByImageTypeName() =\n got: %#v\n want: %#v", gotNames, tt.want)
			}
		})
	}
}

// TestInvalidReposByImageType tests return values from ReposByImageType
// for invalid values
func TestInvalidReposByImageType(t *testing.T) {
	rr := getTestingRepoRegistry()

	ti := test_distro.TestImageType{}

	repos, err := rr.ReposByImageType(&ti)
	assert.Nil(t, repos)
	assert.NotNil(t, err)
}

// TestInvalidreposByImageTypeName tests return values from reposByImageTypeName
// for invalid distro name, arch and image type
func TestInvalidreposByImageTypeName(t *testing.T) {
	rr := getTestingRepoRegistry()

	type args struct {
		distro    string
		arch      string
		imageType string
	}
	tests := []struct {
		name string
		args args
		want func(repos []rpmmd.RepoConfig, err error) bool
	}{
		{
			name: "invalid distro, valid arch and image type",
			args: args{
				distro:    test_distro.TestDistroName + "-invalid",
				arch:      test_distro.TestArchName,
				imageType: test_distro.TestImageTypeName,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
		{
			name: "invalid arch, valid distro and image type",
			args: args{
				distro:    test_distro.TestDistroName,
				arch:      test_distro.TestArchName + "-invalid",
				imageType: test_distro.TestImageTypeName,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
		{
			name: "invalid image type, valid distro and arch, without tagged repos",
			args: args{
				distro:    test_distro.TestDistroName,
				arch:      test_distro.TestArchName,
				imageType: test_distro.TestImageTypeName + "-invalid",
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// a non-empty list of repos should be returned without an error
				if repos == nil || err != nil {
					return false
				}
				// only the list of common distro-arch repos should be returned
				// these are repos without any explicit imageType tag
				if len(repos) != 2 {
					return false
				}
				return true
			},
		},
		{
			name: "invalid image type, valid distro and arch, with tagged repos",
			args: args{
				distro:    test_distro.TestDistroName,
				arch:      test_distro.TestArch2Name,
				imageType: test_distro.TestImageTypeName + "-invalid",
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// a non-empty list of repos should be returned without an error
				if repos == nil || err != nil {
					return false
				}
				// only the list of common distro-arch repos should be returned
				// these are repos without any explicit imageType tag
				if len(repos) != 2 {
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rr.reposByImageTypeName(tt.args.distro, tt.args.arch, tt.args.imageType)
			assert.True(t, tt.want(got, err))
		})
	}
}

func TestReposByArch(t *testing.T) {
	rr := getTestingRepoRegistry()
	testDistro := test_distro.New()

	ta, _ := testDistro.GetArch(test_distro.TestArchName)
	ta2, _ := testDistro.GetArch(test_distro.TestArch2Name)

	type args struct {
		arch        distro.Arch
		taggedRepos bool
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Test Arch 1, without tagged repos",
			args: args{
				arch:        ta,
				taggedRepos: false,
			},
			want: []string{"baseos", "appstream"},
		},
		{
			name: "Test Arch 1, with tagged repos",
			args: args{
				arch:        ta,
				taggedRepos: true,
			},
			want: []string{"baseos", "appstream"},
		},
		{
			name: "Test Arch 2, without tagged repos",
			args: args{
				arch:        ta2,
				taggedRepos: false,
			},
			want: []string{"baseos", "appstream"},
		},
		{
			name: "Test Arch 2, with tagged repos",
			args: args{
				arch:        ta2,
				taggedRepos: true,
			},
			want: []string{"baseos", "appstream", "google-compute-engine", "google-cloud-sdk"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rr.ReposByArchName(tt.args.arch.Distro().Name(), tt.args.arch.Name(), tt.args.taggedRepos)
			assert.Nil(t, err)
			gotNames := []string{}
			for _, r := range got {
				gotNames = append(gotNames, r.Name)
			}

			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Errorf("ReposByArchName() =\n got: %#v\n want: %#v", gotNames, tt.want)
			}
		})
	}
}

// TestInvalidReposByArch tests return values from ReposByArch
// for invalid arch value
func TestInvalidReposByArch(t *testing.T) {
	rr := getTestingRepoRegistry()

	ta := test_distro.TestArch{}
	td := test_distro.TestDistro{}

	repos, err := rr.ReposByArchName(td.Name(), ta.Name(), false)
	assert.Nil(t, repos)
	assert.NotNil(t, err)

	repos, err = rr.ReposByArchName(td.Name(), ta.Name(), false)
	assert.Nil(t, repos)
	assert.NotNil(t, err)
}

// TestInvalidReposByArchName tests return values from ReposByArchName
// for invalid distro name and arch
func TestInvalidReposByArchName(t *testing.T) {
	rr := getTestingRepoRegistry()

	type args struct {
		distro      string
		arch        string
		taggedRepos bool
	}
	tests := []struct {
		name string
		args args
		want func(repos []rpmmd.RepoConfig, err error) bool
	}{
		{
			name: "invalid distro, valid arch, without tagged repos",
			args: args{
				distro:      test_distro.TestDistroName + "-invalid",
				arch:        test_distro.TestArch2Name,
				taggedRepos: false,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
		{
			name: "invalid distro, valid arch, with tagged repos",
			args: args{
				distro:      test_distro.TestDistroName + "-invalid",
				arch:        test_distro.TestArch2Name,
				taggedRepos: true,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
		{
			name: "invalid arch, valid distro, without tagged repos",
			args: args{
				distro:      test_distro.TestDistroName,
				arch:        test_distro.TestArch2Name + "-invalid",
				taggedRepos: false,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
		{
			name: "invalid arch, valid distro, with tagged repos",
			args: args{
				distro:      test_distro.TestDistroName,
				arch:        test_distro.TestArch2Name + "-invalid",
				taggedRepos: true,
			},
			want: func(repos []rpmmd.RepoConfig, err error) bool {
				// the list of repos should be nil and an error should be returned
				if repos != nil || err == nil {
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rr.ReposByArchName(tt.args.distro, tt.args.arch, tt.args.taggedRepos)
			assert.True(t, tt.want(got, err))
		})
	}
}
