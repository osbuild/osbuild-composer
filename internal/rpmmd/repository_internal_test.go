package rpmmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChainPackageSets(t *testing.T) {
	tests := []struct {
		packageSetsChain []string
		packageSets      map[string]PackageSet
		repos            []RepoConfig
		packageSetsRepos map[string][]RepoConfig
		wantChainPkgSets []chainPackageSet
		wantRepos        []RepoConfig
		err              bool
	}{
		// single transaction
		{
			packageSetsChain: []string{"os"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			wantChainPkgSets: []chainPackageSet{
				{
					PackageSet: PackageSet{
						Include: []string{"pkg1"},
						Exclude: []string{"pkg2"},
					},
					Repos: []int{0, 1},
				},
			},
			wantRepos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
		},
		// 2 transactions + package set specific repo
		{
			packageSetsChain: []string{"os", "blueprint"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
				"blueprint": {
					Include: []string{"pkg3"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			packageSetsRepos: map[string][]RepoConfig{
				"blueprint": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
				},
			},
			wantChainPkgSets: []chainPackageSet{
				{
					PackageSet: PackageSet{
						Include: []string{"pkg1"},
						Exclude: []string{"pkg2"},
					},
					Repos: []int{0, 1},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg3"},
					},
					Repos: []int{0, 1, 2},
				},
			},
			wantRepos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
			},
		},
		// 2 transactions + no package set specific repos
		{
			packageSetsChain: []string{"os", "blueprint"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
				"blueprint": {
					Include: []string{"pkg3"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			wantChainPkgSets: []chainPackageSet{
				{
					PackageSet: PackageSet{
						Include: []string{"pkg1"},
						Exclude: []string{"pkg2"},
					},
					Repos: []int{0, 1},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg3"},
					},
					Repos: []int{0, 1},
				},
			},
			wantRepos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
		},
		// 3 transactions + package set specific repo used by 2nd and 3rd transaction
		{
			packageSetsChain: []string{"os", "blueprint", "blueprint2"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
				"blueprint": {
					Include: []string{"pkg3"},
				},
				"blueprint2": {
					Include: []string{"pkg4"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			packageSetsRepos: map[string][]RepoConfig{
				"blueprint": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
				},
				"blueprint2": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
				},
			},
			wantChainPkgSets: []chainPackageSet{
				{
					PackageSet: PackageSet{
						Include: []string{"pkg1"},
						Exclude: []string{"pkg2"},
					},
					Repos: []int{0, 1},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg3"},
					},
					Repos: []int{0, 1, 2},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg4"},
					},
					Repos: []int{0, 1, 2},
				},
			},
			wantRepos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
			},
		},
		// 3 transactions + package set specific repo used by 2nd and 3rd transaction
		// + 3rd transaction using another repo
		{
			packageSetsChain: []string{"os", "blueprint", "blueprint2"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
				"blueprint": {
					Include: []string{"pkg3"},
				},
				"blueprint2": {
					Include: []string{"pkg4"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			packageSetsRepos: map[string][]RepoConfig{
				"blueprint": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
				},
				"blueprint2": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
					{
						Name:    "user-repo-2",
						BaseURL: "https://example.org/user-repo-2",
					},
				},
			},
			wantChainPkgSets: []chainPackageSet{
				{
					PackageSet: PackageSet{
						Include: []string{"pkg1"},
						Exclude: []string{"pkg2"},
					},
					Repos: []int{0, 1},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg3"},
					},
					Repos: []int{0, 1, 2},
				},
				{
					PackageSet: PackageSet{
						Include: []string{"pkg4"},
					},
					Repos: []int{0, 1, 2, 3},
				},
			},
			wantRepos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
				{
					Name:    "user-repo",
					BaseURL: "https://example.org/user-repo",
				},
				{
					Name:    "user-repo-2",
					BaseURL: "https://example.org/user-repo-2",
				},
			},
		},
		// Error: 3 transactions + 3rd one not using repo used by 2nd one
		{
			packageSetsChain: []string{"os", "blueprint", "blueprint2"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
				"blueprint": {
					Include: []string{"pkg3"},
				},
				"blueprint2": {
					Include: []string{"pkg4"},
				},
			},
			repos: []RepoConfig{
				{
					Name:    "baseos",
					BaseURL: "https://example.org/baseos",
				},
				{
					Name:    "appstream",
					BaseURL: "https://example.org/appstream",
				},
			},
			packageSetsRepos: map[string][]RepoConfig{
				"blueprint": {
					{
						Name:    "user-repo",
						BaseURL: "https://example.org/user-repo",
					},
				},
				"blueprint2": {
					{
						Name:    "user-repo2",
						BaseURL: "https://example.org/user-repo2",
					},
				},
			},
			err: true,
		},
		// Error: requested package set name to chain not defined in provided pkg sets
		{
			packageSetsChain: []string{"os", "blueprint"},
			packageSets: map[string]PackageSet{
				"os": {
					Include: []string{"pkg1"},
					Exclude: []string{"pkg2"},
				},
			},
			err: true,
		},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			gotChainPackageSets, gotRepos, err := chainPackageSets(tt.packageSetsChain, tt.packageSets, tt.repos, tt.packageSetsRepos)
			if tt.err {
				assert.NotNilf(t, err, "expected an error, but got 'nil' instead")
				assert.Nilf(t, gotChainPackageSets, "got non-nill []rpmmd.ChainPackageSet, but expected an error")
				assert.Nilf(t, gotRepos, "got non-nill []rpmmd.RepoConfig, but expected an error")
			} else {
				assert.Nilf(t, err, "expected 'nil', but got error instead")
				assert.NotNilf(t, gotChainPackageSets, "expected non-nill []rpmmd.ChainPackageSet, but got 'nil' instead")
				assert.NotNilf(t, gotRepos, "expected non-nill []rpmmd.RepoConfig, but got 'nil' instead")

				assert.Equal(t, tt.wantChainPkgSets, gotChainPackageSets)
				assert.Equal(t, tt.wantRepos, gotRepos)
			}
		})
	}
}
