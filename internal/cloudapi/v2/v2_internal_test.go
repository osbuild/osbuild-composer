package v2

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			Baseurl:     common.StringToPtr("http://example.com/baseos-rhel7"), // only build
			PackageSets: &[]string{"build"},
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/extra-tools"), // build and archive
			PackageSets: &[]string{"build", "archive"},
		},
		{
			Baseurl:     common.StringToPtr("http://example.com/custom-os-stuff"), // blueprint -> merge with custom
			PackageSets: &[]string{"blueprint"},
		},
	}

	expectedPkgSetRepos := map[string][]rpmmd.RepoConfig{
		"build": {
			{
				BaseURL: "http://example.com/baseos-rhel7",
			},
			{
				BaseURL: "http://example.com/extra-tools",
			},
		},
		"archive": {
			{
				BaseURL: "http://example.com/extra-tools",
			},
		},
		"blueprint": {
			{
				BaseURL: "http://example.com/custom-os-stuff",
			},
			{
				BaseURL: "http://example.com/repoone",
			},
			{
				BaseURL: "http://example.com/repotwo",
			},
		},
	}

	payloadPkgSets := []string{"blueprint"}

	baseRepos, pkgSetRepos, err := collectRepos(irRepos, customRepos, payloadPkgSets)

	// check lengths
	assert.NoError(err)
	assert.Len(baseRepos, 2)
	assert.Len(pkgSetRepos, 3)

	// check tags in package set repo map
	for _, tag := range []string{"blueprint", "build", "archive"} {
		assert.Contains(pkgSetRepos, tag)
	}
	assert.NotContains(pkgSetRepos, "should-be-ignored")

	// check URLs
	baseRepoURLs := make([]string, len(baseRepos))
	for idx, baseRepo := range baseRepos {
		baseRepoURLs[idx] = baseRepo.BaseURL
	}
	for _, url := range []string{"http://example.com/baseos", "http://example.com/appstream"} {
		assert.Contains(baseRepoURLs, url)
	}

	assert.Equal(pkgSetRepos, expectedPkgSetRepos)
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
				GpgKey:      common.StringToPtr("some-kind-of-key"),
				IgnoreSsl:   common.BoolToPtr(false),
				Metalink:    nil,
				Mirrorlist:  nil,
				Rhsm:        false,
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
				GpgKey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  common.StringToPtr("http://example.org/mirrorlist"),
				Rhsm:        false,
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
				GpgKey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  common.StringToPtr("http://example.org/mirrorlist"),
				Rhsm:        false,
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
				GpgKey:      nil,
				IgnoreSsl:   common.BoolToPtr(true),
				Metalink:    common.StringToPtr("http://example.org/metalink"),
				Mirrorlist:  nil,
				Rhsm:        true,
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

	// test error
	noURL := Repository{
		Baseurl:     nil,
		CheckGpg:    nil,
		GpgKey:      nil,
		IgnoreSsl:   nil,
		Metalink:    nil,
		Mirrorlist:  nil,
		Rhsm:        true,
		PackageSets: nil,
	}
	_, err := genRepoConfig(noURL)
	assert.EqualError(err, HTTPError(ErrorInvalidRepository).Error())
}
