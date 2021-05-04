package reporegistry

import (
	"fmt"
	"reflect"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// RepoRegistry represents a database of distro and architecture
// specific RPM repositories. Image types are considered only
// if the loaded repository definition contains any ImageTypeTags.
type RepoRegistry struct {
	repos rpmmd.DistrosRepoConfigs
}

// New returns a new RepoRegistry instance with the data
// loaded from the given repoConfigPaths
func New(repoConfigPaths []string) (*RepoRegistry, error) {
	repositories, err := rpmmd.LoadAllRepositories(repoConfigPaths)
	if err != nil {
		return nil, err
	}

	return &RepoRegistry{repositories}, nil
}

// ReposByImageType returns a slice of rpmmd.RepoConfig instances, which should be used for building the specific
// image type. All repositories for the associated distribution and architecture, without any ImageTypeTags set,
// are always part of the returned slice. In addition, if there are repositories tagged with the specific image
// type name, these are added to the returned slice as well.
func (r *RepoRegistry) ReposByImageType(imageType distro.ImageType) ([]rpmmd.RepoConfig, error) {
	if imageType.Arch() == nil || reflect.ValueOf(imageType.Arch()).IsNil() {
		return nil, fmt.Errorf("there is no architecture associated with the provided image type")
	}
	if imageType.Arch().Distro() == nil || reflect.ValueOf(imageType.Arch().Distro()).IsNil() {
		return nil, fmt.Errorf("there is no distribution associated with the architecture associated with the provided image type")
	}
	return r.reposByImageTypeName(imageType.Arch().Distro().Name(), imageType.Arch().Name(), imageType.Name())
}

// reposByImageTypeName returns a slice of rpmmd.RepoConfig instances, which should be used for building the specific
// image type name (of a given distribution and architecture). The method does not verify
// if the given image type name is actually part of the architecture definition of the provided name.
// Therefore in general, all common distro-arch-specific repositories are returned for any image type name,
// even for non-existing ones.
func (r *RepoRegistry) reposByImageTypeName(distro, arch, imageType string) ([]rpmmd.RepoConfig, error) {
	repositories := []rpmmd.RepoConfig{}

	distroRepos, found := r.repos[distro]
	if !found {
		return nil, fmt.Errorf("there are no repositories for distribution '%s'", distro)
	}
	archRepos, found := distroRepos[arch]
	if !found {
		return nil, fmt.Errorf("there are no repositories for distribution '%s' and architecture '%s'", distro, arch)
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
