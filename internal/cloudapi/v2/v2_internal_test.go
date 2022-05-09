package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func TestSplitExtension(t *testing.T) {
	tests := []struct {
		filename  string
		extension string
	}{
		{filename: "image.qcow2", extension: ".qcow2"},
		{filename: "image.tar.gz", extension: ".tar.gz"},
		{filename: "", extension: ""},
		{filename: ".htaccess", extension: ""},
		{filename: ".weirdfile.txt", extension: ".txt"},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			require.Equal(t, tt.extension, splitExtension(tt.filename))
		})
	}
}

func TestCollectRepos(t *testing.T) {
	assert := assert.New(t)

	// user repositories from request customizations
	customRepos := []Repository{
		{
			Baseurl: common.StringToPtr("http://example.com/repoone"),
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/repotwo"),
			PackageSets: &[]string{"should-be-ignored"},
		},
	}

	// repos from the image request (standard repos + package set repos)
	irRepos := []Repository{
		{
			Baseurl: common.StringToPtr("http://example.com/baseos"), // empty field -> all package sets
		},
		{
			Baseurl: common.StringToPtr("http://example.com/appstream"), // empty field -> all package sets
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/baseos-rhel7"), // build only
			PackageSets: &[]string{"build"},
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/extra-tools"), // build and archive
			PackageSets: &[]string{"build", "archive"},
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/custom-os-stuff"), // blueprint only
			PackageSets: &[]string{"blueprint"},
		},
	}

	expectedRepos := []rpmmd.RepoConfig{
		{BaseURL: "http://example.com/baseos", PackageSets: nil},
		{BaseURL: "http://example.com/appstream", PackageSets: nil},
		{BaseURL: "http://example.com/baseos-rhel7", PackageSets: []string{"build"}},
		{BaseURL: "http://example.com/extra-tools", PackageSets: []string{"build", "archive"}},
		{BaseURL: "http://example.com/custom-os-stuff", PackageSets: []string{"blueprint"}},
		{BaseURL: "http://example.com/repoone", PackageSets: []string{"blueprint"}},
		{BaseURL: "http://example.com/repotwo", PackageSets: []string{"blueprint"}},
	}

	payloadPkgSets := []string{"blueprint"}

	repos, err := convertRepos(irRepos, customRepos, payloadPkgSets)

	// check lengths
	assert.NoError(err)
	assert.Equal(repos, expectedRepos)
}

func TestRepoConfigConversion(t *testing.T) {
	assert := assert.New(t)
	type testCase struct {
		repo       Repository
		repoConfig rpmmd.RepoConfig
	}

	testCases := []testCase{
		{
			repo: Repository{
				Baseurl:     common.StringToPtr("http://base.url"),
				CheckGpg:    common.BoolToPtr(true),
				Gpgkey:      common.StringToPtr("some-kind-of-key"),
				IgnoreSsl:   common.BoolToPtr(false),
				Metalink:    nil,
				Mirrorlist:  nil,
				Rhsm:        common.BoolToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURL:        "http://base.url",
				Metalink:       "",
				MirrorList:     "",
				GPGKey:         "some-kind-of-key",
				CheckGPG:       true,
				IgnoreSSL:      false,
				MetadataExpire: "",
				RHSM:           false,
				ImageTypeTags:  nil,
			},
		},
		{
			repo: Repository{
				Baseurl:     common.StringToPtr("http://base.url"),
				CheckGpg:    nil,
				Gpgkey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  common.StringToPtr("http://example.org/mirrorlist"),
				Rhsm:        common.BoolToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURL:        "http://base.url",
				Metalink:       "", // since BaseURL is specified, MetaLink is not copied
				MirrorList:     "", // since BaseURL is specified, MirrorList is not copied
				GPGKey:         "",
				CheckGPG:       false,
				IgnoreSSL:      true,
				MetadataExpire: "",
				RHSM:           false,
				ImageTypeTags:  nil,
			},
		},
		{
			repo: Repository{
				Baseurl:     nil,
				CheckGpg:    nil,
				Gpgkey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  common.StringToPtr("http://example.org/mirrorlist"),
				Rhsm:        common.BoolToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURL:        "",
				Metalink:       "", // since MirrorList is specified, MetaLink is not copied
				MirrorList:     "http://example.org/mirrorlist",
				GPGKey:         "",
				CheckGPG:       false,
				IgnoreSSL:      true,
				MetadataExpire: "",
				RHSM:           false,
				ImageTypeTags:  nil,
			},
		},
		{
			repo: Repository{
				Baseurl:     nil,
				CheckGpg:    nil,
				Gpgkey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  nil,
				Rhsm:        common.BoolToPtr(true),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURL:        "",
				Metalink:       "http://example.org/metalink",
				MirrorList:     "",
				GPGKey:         "",
				CheckGPG:       false,
				IgnoreSSL:      true,
				MetadataExpire: "",
				RHSM:           true,
				ImageTypeTags:  nil,
			},
		},
	}

	for idx, tc := range testCases {
		rc, err := genRepoConfig(tc.repo)
		assert.NoError(err)
		assert.Equal(rc, &tc.repoConfig, "mismatch in test case %d", idx)
	}

	errorTestCases := []struct {
		repo Repository
		err  string
	}{
		// invalid repo
		{
			repo: Repository{
				Baseurl:     nil,
				CheckGpg:    nil,
				Gpgkey:      nil,
				IgnoreSsl:   nil,
				Metalink:    nil,
				Mirrorlist:  nil,
				Rhsm:        common.BoolToPtr(true),
				PackageSets: nil,
			},
			err: HTTPError(ErrorInvalidRepository).Error(),
		},

		// check gpg required but no gpgkey given
		{
			repo: Repository{
				Baseurl:     nil,
				CheckGpg:    common.BoolToPtr(true),
				Gpgkey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  nil,
				Rhsm:        common.BoolToPtr(true),
				PackageSets: nil,
			},
			err: HTTPError(ErrorNoGPGKey).Error(),
		},
	}

	for _, tc := range errorTestCases {
		rc, err := genRepoConfig(tc.repo)
		assert.Nil(rc)
		assert.EqualError(err, tc.err)
	}
}
