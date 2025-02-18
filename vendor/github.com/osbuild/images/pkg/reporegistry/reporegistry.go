package reporegistry

import (
	"fmt"
	"io/fs"

	"github.com/osbuild/images/pkg/distroidparser"
	"github.com/osbuild/images/pkg/rpmmd"
)

// RepoRegistry represents a database of distro and architecture
// specific RPM repositories. Image types are considered only
// if the loaded repository definition contains any ImageTypeTags.
type RepoRegistry struct {
	repos rpmmd.DistrosRepoConfigs
}

// New returns a new RepoRegistry instance with the data loaded from
// the given repoConfigPaths and repoConfigFS instance. The order is
// important here, first the paths are tried, then the FSes.
//
// Note that the confPaths must point directly to the directory with
// the json repo files.
func New(repoConfigPaths []string, repoConfigFS []fs.FS) (*RepoRegistry, error) {
	repositories, err := LoadAllRepositories(repoConfigPaths, repoConfigFS)
	if err != nil {
		return nil, err
	}

	return &RepoRegistry{repositories}, nil
}

func NewFromDistrosRepoConfigs(distrosRepoConfigs rpmmd.DistrosRepoConfigs) *RepoRegistry {
	return &RepoRegistry{distrosRepoConfigs}
}

// ReposByImageTypeName returns a slice of rpmmd.RepoConfig instances, which should be used for building the specific
// image type name (of a given distribution and architecture). The method does not verify
// if the given image type name is actually part of the architecture definition of the provided name.
// Therefore in general, all common distro-arch-specific repositories are returned for any image type name,
// even for non-existing ones.
func (r *RepoRegistry) ReposByImageTypeName(distro, arch, imageType string) ([]rpmmd.RepoConfig, error) {
	repositories := []rpmmd.RepoConfig{}

	archRepos, err := r.ReposByArchName(distro, arch, true)
	if err != nil {
		return nil, err
	}

	for _, repo := range archRepos {
		// Add all repositories without image_type tags
		if len(repo.ImageTypeTags) == 0 {
			repositories = append(repositories, repo)
			continue
		}

		// Add all repositories tagged with the image type
		for _, imageNameTag := range repo.ImageTypeTags {
			if imageNameTag == imageType {
				repositories = append(repositories, repo)
				break
			}
		}
	}

	return repositories, nil
}

// reposByArchName returns a slice of rpmmd.RepoConfig instances, which should be used for building image types for the
// specific architecture and distribution. This includes by default all repositories without any image type tags specified.
// Depending on the `includeTagged` argument value, repositories with image type tags set will be added to the returned
// slice or not.
//
// The method does not verify if the given architecture name is actually part of the specific distribution definition.
func (r *RepoRegistry) ReposByArchName(distro, arch string, includeTagged bool) ([]rpmmd.RepoConfig, error) {
	repositories := []rpmmd.RepoConfig{}

	archRepos, err := r.DistroHasRepos(distro, arch)
	if err != nil {
		return nil, fmt.Errorf("Failed to get repositories for distribution '%s' and architecture '%s': %v", distro, arch, err)
	}

	for _, repo := range archRepos {
		// skip repos with image type tags if specified to do so
		if !includeTagged && len(repo.ImageTypeTags) != 0 {
			continue
		}

		repositories = append(repositories, repo)
	}

	return repositories, nil
}

// DistroHasRepos returns the repositories for the distro+arch, and a found flag
func (r *RepoRegistry) DistroHasRepos(distro, arch string) ([]rpmmd.RepoConfig, error) {
	// compatibility layer to support old repository definition filenames
	// without a dot to separate major and minor release versions
	stdDistroName, err := distroidparser.DefaultParser.Standardize(distro)
	if err != nil {
		return nil, fmt.Errorf("failed to parse distro ID string: %v", err)
	}

	distroRepos, found := r.repos[stdDistroName]
	if !found {
		return nil, fmt.Errorf("there are no repositories for distribution '%s'", stdDistroName)
	}
	repos, found := distroRepos[arch]
	if !found {
		return nil, fmt.Errorf("there are no repositories for distribution '%s' and architecture '%s'", stdDistroName, arch)
	}

	return repos, nil
}

// ListDistros returns a list of all distros which have a repository defined
// in the registry.
func (r *RepoRegistry) ListDistros() []string {
	distros := make([]string, 0, len(r.repos))
	for name := range r.repos {
		distros = append(distros, name)
	}
	return distros
}
