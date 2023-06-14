package blueprint

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/fsnode"
	"github.com/stretchr/testify/assert"
)

func TestGetCustomRepositories(t *testing.T) {
	testCases := []struct {
		name                   string
		expectedCustomizations Customizations
		wantErr                error
	}{
		{
			name: "Test no errors",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
					},
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "Test empty id error",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{},
				},
			},
			wantErr: fmt.Errorf("Repository ID is required"),
		},
		{
			name: "Test empty baseurl, mirrorlist or metalink error",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{
						Id: "example-1",
					},
				},
			},
			wantErr: fmt.Errorf("Repository base URL, mirrorlist or metalink is required"),
		},
		{
			name: "Test missing GPG keys error",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGCheck: common.ToPtr(true),
					},
				},
			},
			wantErr: fmt.Errorf("Repository gpg check is set to true but no gpg keys are provided"),
		},
		{
			name: "Test invalid GPG keys error",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{"invalid"},
						GPGCheck: common.ToPtr(true),
					},
				},
			},
			wantErr: fmt.Errorf("Repository gpg key is not a valid URL or a valid gpg key"),
		},
		{
			name: "Test invalid repository filename error",
			expectedCustomizations: Customizations{
				Repositories: []RepositoryCustomization{
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						Filename: "!nval!d",
					},
				},
			},
			wantErr: fmt.Errorf("Repository filename %q is invalid", "!nval!d.repo"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				retCustomizations, err := tt.expectedCustomizations.GetRepositories()
				assert.NoError(t, err)
				assert.EqualValues(t, tt.expectedCustomizations.Repositories, retCustomizations)
			} else {
				_, err := tt.expectedCustomizations.GetRepositories()
				assert.Equal(t, tt.wantErr, err)
			}
		})
	}
}

func TestCustomRepoFilename(t *testing.T) {
	testCases := []struct {
		Name         string
		Repo         RepositoryCustomization
		WantFilename string
	}{
		{
			Name: "Test default filename #1",
			Repo: RepositoryCustomization{
				Id:       "example-1",
				BaseURLs: []string{"http://example-1.com"},
			},
			WantFilename: "example-1.repo",
		},
		{
			Name: "Test default filename #2",
			Repo: RepositoryCustomization{
				Id:       "example-2",
				BaseURLs: []string{"http://example-1.com"},
			},
			WantFilename: "example-2.repo",
		},
		{
			Name: "Test custom filename",
			Repo: RepositoryCustomization{
				Id:       "example-1",
				BaseURLs: []string{"http://example-1.com"},
				Filename: "test.repo",
			},
			WantFilename: "test.repo",
		},
		{
			Name: "Test custom filename without extension",
			Repo: RepositoryCustomization{
				Id:       "example-1",
				BaseURLs: []string{"http://example-1.com"},
				Filename: "test",
			},
			WantFilename: "test.repo",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			got := tt.Repo.getFilename()
			assert.Equal(t, tt.WantFilename, got)
		})
	}
}

func TestCustomRepoToRepoConfigAndGPGKeys(t *testing.T) {
	ensureFileCreation := func(file *fsnode.File, err error) *fsnode.File {
		t.Helper()
		assert.NoError(t, err)
		assert.NotNil(t, file)
		return file
	}
	testCases := []struct {
		Name           string
		Repos          []RepositoryCustomization
		WantRepoConfig map[string][]rpmmd.RepoConfig
		WantGPGKeys    []*fsnode.File
	}{
		{
			Name: "Test no gpg keys, no filenames",
			Repos: []RepositoryCustomization{
				{
					Id:       "example-1",
					BaseURLs: []string{"http://example-1.com"},
				},
				{
					Id:       "example-2",
					BaseURLs: []string{"http://example-2.com"},
				},
			},
			WantRepoConfig: map[string][]rpmmd.RepoConfig{
				"example-1.repo": {
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{},
					},
				},
				"example-2.repo": {
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
						GPGKeys:  []string{},
					},
				},
			},
			WantGPGKeys: nil,
		},
		{
			Name: "Test no gpg keys, filenames",
			Repos: []RepositoryCustomization{
				{
					Id:       "example-1",
					BaseURLs: []string{"http://example-1.com"},
					Filename: "test-1.repo",
				},
				{
					Id:       "example-2",
					BaseURLs: []string{"http://example-2.com"},
					Filename: "test-2.repo",
				},
			},
			WantRepoConfig: map[string][]rpmmd.RepoConfig{
				"test-1.repo": {
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{},
					},
				},
				"test-2.repo": {
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
						GPGKeys:  []string{},
					},
				},
			},
			WantGPGKeys: nil,
		},
		{
			Name: "Test remote gpgkeys",
			Repos: []RepositoryCustomization{
				{
					Id:       "example-1",
					BaseURLs: []string{"http://example-1.com"},
					GPGKeys:  []string{"http://example-1.com/gpgkey"},
					GPGCheck: common.ToPtr(true),
				},
				{
					Id:       "example-2",
					BaseURLs: []string{"http://example-2.com"},
					GPGKeys:  []string{"http://example-2.com/gpgkey"},
					GPGCheck: common.ToPtr(true),
				},
			},
			WantRepoConfig: map[string][]rpmmd.RepoConfig{
				"example-1.repo": {
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{"http://example-1.com/gpgkey"},
						CheckGPG: common.ToPtr(true),
					},
				},
				"example-2.repo": {
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
						GPGKeys:  []string{"http://example-2.com/gpgkey"},
						CheckGPG: common.ToPtr(true),
					},
				},
			},
			WantGPGKeys: nil,
		},
		{
			Name: "Test inline gpgkeys",
			Repos: []RepositoryCustomization{
				{
					Id:       "example-1",
					BaseURLs: []string{"http://example-1.com"},
					GPGKeys:  []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n"},
					GPGCheck: common.ToPtr(true),
				},
				{
					Id:       "example-2",
					BaseURLs: []string{"http://example-2.com"},
					GPGKeys:  []string{"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-2-----END PGP PUBLIC KEY BLOCK-----\n"},
					GPGCheck: common.ToPtr(true),
				},
			},
			WantRepoConfig: map[string][]rpmmd.RepoConfig{
				"example-1.repo": {
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-0"},
						CheckGPG: common.ToPtr(true),
					},
				},
				"example-2.repo": {
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
						GPGKeys:  []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-0"},
						CheckGPG: common.ToPtr(true),
					},
				},
			},
			WantGPGKeys: []*fsnode.File{
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-0", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n"))),
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-0", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n"))),
			},
		},
		{
			Name: "Test multiple inline gpgkeys",
			Repos: []RepositoryCustomization{
				{
					Id:       "example-1",
					BaseURLs: []string{"http://example-1.com"},
					GPGKeys: []string{
						"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n",
						"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-2-----END PGP PUBLIC KEY BLOCK-----\n",
					},
					GPGCheck: common.ToPtr(true),
				},
				{
					Id:       "example-2",
					BaseURLs: []string{"http://example-2.com"},
					GPGKeys: []string{
						"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n",
						"-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-2-----END PGP PUBLIC KEY BLOCK-----\n",
					},
					GPGCheck: common.ToPtr(true),
				},
			},
			WantRepoConfig: map[string][]rpmmd.RepoConfig{
				"example-1.repo": {
					{
						Id:       "example-1",
						BaseURLs: []string{"http://example-1.com"},
						GPGKeys:  []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-0", "file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-1"},
						CheckGPG: common.ToPtr(true),
					},
				},
				"example-2.repo": {
					{
						Id:       "example-2",
						BaseURLs: []string{"http://example-2.com"},
						GPGKeys:  []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-0", "file:///etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-1"},
						CheckGPG: common.ToPtr(true),
					},
				},
			},
			WantGPGKeys: []*fsnode.File{
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-0", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n"))),
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-1-1", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-2-----END PGP PUBLIC KEY BLOCK-----\n"))),
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-0", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-1-----END PGP PUBLIC KEY BLOCK-----\n"))),
				ensureFileCreation(fsnode.NewFile("/etc/pki/rpm-gpg/RPM-GPG-KEY-example-2-1", nil, nil, nil, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----fake-gpg-key-2-----END PGP PUBLIC KEY BLOCK-----\n"))),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			got, _, err := RepoCustomizationsToRepoConfigAndGPGKeyFiles(tt.Repos)
			assert.NoError(t, err)
			assert.Equal(t, tt.WantRepoConfig, got)
		})
	}
}
