package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/common"
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
			Baseurl: common.ToPtr("http://example.com/repoone"),
		},
		{
			Baseurl:     common.ToPtr("http://example.com/repotwo"),
			PackageSets: &[]string{"should-be-ignored"},
		},
	}

	// repos from the image request (standard repos + package set repos)
	irRepos := []Repository{
		{
			Baseurl: common.ToPtr("http://example.com/baseos"), // empty field -> all package sets
		},
		{
			Baseurl: common.ToPtr("http://example.com/appstream"), // empty field -> all package sets
		},
		{
			Baseurl:     common.ToPtr("http://example.com/baseos-rhel7"), // build only
			PackageSets: &[]string{"build"},
		},
		{
			Baseurl:     common.ToPtr("http://example.com/extra-tools"), // build and archive
			PackageSets: &[]string{"build", "archive"},
		},
		{
			Baseurl:     common.ToPtr("http://example.com/custom-os-stuff"), // blueprint only
			PackageSets: &[]string{"blueprint"},
		},
	}

	expectedRepos := []rpmmd.RepoConfig{
		{BaseURLs: []string{"http://example.com/baseos"}, PackageSets: nil},
		{BaseURLs: []string{"http://example.com/appstream"}, PackageSets: nil},
		{BaseURLs: []string{"http://example.com/baseos-rhel7"}, PackageSets: []string{"build"}},
		{BaseURLs: []string{"http://example.com/extra-tools"}, PackageSets: []string{"build", "archive"}},
		{BaseURLs: []string{"http://example.com/custom-os-stuff"}, PackageSets: []string{"blueprint"}},
		{BaseURLs: []string{"http://example.com/repoone"}, PackageSets: []string{"blueprint"}},
		{BaseURLs: []string{"http://example.com/repotwo"}, PackageSets: []string{"blueprint"}},
	}

	payloadPkgSets := []string{"blueprint"}

	repos, err := convertRepos(irRepos, customRepos, payloadPkgSets)
	assert.NoError(err)

	// check lengths
	assert.NoError(err)
	assert.Equal(expectedRepos, repos)
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
				Baseurl:     common.ToPtr("http://base.url"),
				CheckGpg:    common.ToPtr(true),
				Gpgkey:      common.ToPtr("some-kind-of-key"),
				IgnoreSsl:   common.ToPtr(false),
				Metalink:    nil,
				Mirrorlist:  nil,
				Rhsm:        common.ToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURLs:       []string{"http://base.url"},
				Metalink:       "",
				MirrorList:     "",
				GPGKeys:        []string{"some-kind-of-key"},
				CheckGPG:       common.ToPtr(true),
				IgnoreSSL:      common.ToPtr(false),
				MetadataExpire: "",
				RHSM:           false,
				ImageTypeTags:  nil,
			},
		},
		{
			repo: Repository{
				Baseurl:     common.ToPtr("http://base.url"),
				CheckGpg:    nil,
				Gpgkey:      nil,
				IgnoreSsl:   common.ToPtr(true),
				Metalink:    common.ToPtr("http://example.org/metalink"),
				Mirrorlist:  common.ToPtr("http://example.org/mirrorlist"),
				Rhsm:        common.ToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				BaseURLs:       []string{"http://base.url"},
				Metalink:       "", // since BaseURL is specified, MetaLink is not copied
				MirrorList:     "", // since BaseURL is specified, MirrorList is not copied
				CheckGPG:       nil,
				IgnoreSSL:      common.ToPtr(true),
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
				IgnoreSsl:   common.ToPtr(true),
				Metalink:    common.ToPtr("http://example.org/metalink"),
				Mirrorlist:  common.ToPtr("http://example.org/mirrorlist"),
				Rhsm:        common.ToPtr(false),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				Metalink:       "", // since MirrorList is specified, MetaLink is not copied
				MirrorList:     "http://example.org/mirrorlist",
				CheckGPG:       nil,
				IgnoreSSL:      common.ToPtr(true),
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
				IgnoreSsl:   common.ToPtr(true),
				Metalink:    common.ToPtr("http://example.org/metalink"),
				Mirrorlist:  nil,
				Rhsm:        common.ToPtr(true),
				PackageSets: nil,
			},
			repoConfig: rpmmd.RepoConfig{
				Name:           "",
				Metalink:       "http://example.org/metalink",
				MirrorList:     "",
				CheckGPG:       nil,
				IgnoreSSL:      common.ToPtr(true),
				MetadataExpire: "",
				RHSM:           true,
				ImageTypeTags:  nil,
			},
		},
	}

	for idx, tc := range testCases {
		repoConfig := tc.repoConfig
		rc, err := genRepoConfig(tc.repo)
		assert.NoError(err)
		assert.Equal(&repoConfig, rc, "mismatch in test case %d", idx)
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
				Rhsm:        common.ToPtr(true),
				PackageSets: nil,
			},
			err: HTTPError(ErrorInvalidRepository).Error(),
		},

		// check gpg required but no gpgkey given
		{
			repo: Repository{
				CheckGpg:    common.ToPtr(true),
				Gpgkey:      nil,
				IgnoreSsl:   common.ToPtr(true),
				Metalink:    common.ToPtr("http://example.org/metalink"),
				Mirrorlist:  nil,
				Rhsm:        common.ToPtr(true),
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

func TestStagesToPackageMetadata(t *testing.T) {
	assert := assert.New(t)
	type testCase struct {
		stages []osbuild.RPMStageMetadata
		pkgs   []PackageMetadata
	}
	testCases := []testCase{
		{
			stages: []osbuild.RPMStageMetadata{
				{
					Packages: []osbuild.RPMPackageMetadata{
						{
							Name:    "vim-minimal",
							Version: "8.0.1763",
							Release: "15.el8",
							Epoch:   common.ToPtr("2"),
							Arch:    "x86_64",
							SigMD5:  "v",
							SigPGP:  "v",
							SigGPG:  "v",
						},
						{
							Name:    "unique",
							Version: "1.90",
							Release: "10",
							Epoch:   nil,
							Arch:    "aarch64",
							SigMD5:  "v",
							SigPGP:  "v",
							SigGPG:  "v",
						},
					},
				},
			},
			pkgs: []PackageMetadata{
				{
					Type:      "rpm",
					Name:      "vim-minimal",
					Version:   "8.0.1763",
					Release:   "15.el8",
					Epoch:     common.ToPtr("2"),
					Arch:      "x86_64",
					Sigmd5:    common.ToPtr("v"),
					Signature: common.ToPtr("v"),
				},
				{
					Type:      "rpm",
					Name:      "unique",
					Version:   "1.90",
					Release:   "10",
					Epoch:     nil,
					Arch:      "aarch64",
					Sigmd5:    common.ToPtr("v"),
					Signature: common.ToPtr("v"),
				},
			},
		},
		{
			stages: []osbuild.RPMStageMetadata{
				{
					Packages: []osbuild.RPMPackageMetadata{
						{
							Name:    "vim-minimal",
							Version: "8.0.1763",
							Release: "15.el8",
							Epoch:   common.ToPtr("2"),
							Arch:    "x86_64",
							SigMD5:  "v",
							SigPGP:  "v",
							SigGPG:  "v",
						},
					},
				},
				{
					Packages: []osbuild.RPMPackageMetadata{
						{
							Name:    "unique",
							Version: "1.90",
							Release: "10",
							Epoch:   nil,
							Arch:    "aarch64",
							SigMD5:  "v",
							SigPGP:  "v",
							SigGPG:  "v",
						},
					},
				},
			},
			pkgs: []PackageMetadata{
				{
					Type:      "rpm",
					Name:      "vim-minimal",
					Version:   "8.0.1763",
					Release:   "15.el8",
					Epoch:     common.ToPtr("2"),
					Arch:      "x86_64",
					Sigmd5:    common.ToPtr("v"),
					Signature: common.ToPtr("v"),
				},
				{
					Type:      "rpm",
					Name:      "unique",
					Version:   "1.90",
					Release:   "10",
					Epoch:     nil,
					Arch:      "aarch64",
					Sigmd5:    common.ToPtr("v"),
					Signature: common.ToPtr("v"),
				},
			},
		},
	}

	for idx, tc := range testCases {
		assert.Equal(tc.pkgs, stagesToPackageMetadata(tc.stages), "mismatch in test case %d", idx)
	}
}

func TestPackageSpecToPackageMetadata(t *testing.T) {
	assert := assert.New(t)
	type testCase struct {
		specs []rpmmd.PackageSpec
		pkgs  []PackageMetadata
	}
	testCases := []testCase{
		{
			specs: []rpmmd.PackageSpec{
				{
					Name:     "vim-minimal",
					Version:  "8.0.1763",
					Release:  "15.el8",
					Epoch:    2,
					Arch:     "x86_64",
					Checksum: "sha256:HASH",
				},
				{
					Name:     "unique",
					Version:  "1.90",
					Release:  "10",
					Epoch:    0,
					Arch:     "aarch64",
					Checksum: "sha256:HASH",
				},
			},
			pkgs: []PackageMetadata{
				{
					Type:     "rpm",
					Name:     "vim-minimal",
					Version:  "8.0.1763",
					Release:  "15.el8",
					Epoch:    common.ToPtr("2"),
					Arch:     "x86_64",
					Checksum: common.ToPtr("sha256:HASH"),
				},
				{
					Type:     "rpm",
					Name:     "unique",
					Version:  "1.90",
					Release:  "10",
					Epoch:    nil,
					Arch:     "aarch64",
					Checksum: common.ToPtr("sha256:HASH"),
				},
			},
		},
	}

	for idx, tc := range testCases {
		assert.Equal(tc.pkgs, packageSpecToPackageMetadata(tc.specs), "mismatch in test case %d", idx)
	}
}

func TestPackageInfoToPackageInfo(t *testing.T) {
	assert := assert.New(t)
	type testCase struct {
		pkgs []rpmmd.PackageInfo
		info []PackageInfo
	}
	testCases := []testCase{
		{
			pkgs: []rpmmd.PackageInfo{
				{
					Name:        "vim-enhanced",
					Summary:     "A version of the VIM editor which includes recent enhancements",
					Description: "VIM (VIsual editor iMproved) is an updated and improved ...",
					Homepage:    "http://www.vim.org/",
					Builds: []rpmmd.PackageBuild{
						{
							Arch:      "x86_64",
							BuildTime: "2024-09-06T16:14:20",
							Epoch:     2,
							Release:   "1.fc40",
							Source: rpmmd.PackageSource{
								Version: "9.1.719",
								License: "Vim AND LGPL-2.1-or-later AND ...",
							},
						},
					},
				},
			},
			info: []PackageInfo{
				{
					Name:        "vim-enhanced",
					Summary:     "A version of the VIM editor which includes recent enhancements",
					Description: common.ToPtr("VIM (VIsual editor iMproved) is an updated and improved ..."),
					Homepage:    common.ToPtr("http://www.vim.org/"),
					Builds: &[]PackageBuild{
						{
							Arch:      "x86_64",
							BuildTime: common.ToPtr("2024-09-06T16:14:20"),
							Epoch:     common.ToPtr("2"),
							Release:   "1.fc40",
							Source: PackageSource{
								Version: "9.1.719",
								License: "Vim AND LGPL-2.1-or-later AND ...",
							},
						},
					},
				},
			},
		},
	}

	for idx, tc := range testCases {
		assert.Equal(tc.info, packageInfoToPackageInfo(tc.pkgs), "mismatch in test case %d", idx)
	}
}
