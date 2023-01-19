package blueprint

import (
	"fmt"
	"testing"

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
