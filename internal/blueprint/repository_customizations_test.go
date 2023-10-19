package blueprint

import (
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
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
