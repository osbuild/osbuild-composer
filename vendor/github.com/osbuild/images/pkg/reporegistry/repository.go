package reporegistry

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/distroidparser"
	"github.com/osbuild/images/pkg/rpmmd"
)

// LoadAllRepositories loads all repositories for given distros from the given list of paths.
// Behavior is the same as with the LoadRepositories() method.
func LoadAllRepositories(confPaths []string) (rpmmd.DistrosRepoConfigs, error) {
	distrosRepoConfigs := rpmmd.DistrosRepoConfigs{}

	for _, confPath := range confPaths {
		reposPath := filepath.Join(confPath, "repositories")

		fileEntries, err := os.ReadDir(reposPath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, fileEntry := range fileEntries {
			// Skip all directories
			if fileEntry.IsDir() {
				continue
			}

			// distro repositories definition is expected to be named "<distro_name>.json"
			if strings.HasSuffix(fileEntry.Name(), ".json") {
				distroIDStr := strings.TrimSuffix(fileEntry.Name(), ".json")

				// compatibility layer to support old repository definition filenames
				// without a dot to separate major and minor release versions
				distro, err := distroidparser.DefaultParser.Standardize(distroIDStr)
				if err != nil {
					log.Printf("failed to parse distro ID string, using it as is: %v", err)
					// NB: Before the introduction of distro ID standardization, the filename
					//     was used as the distro ID. This is kept for backward compatibility
					//     if the filename can't be parsed.
					distro = distroIDStr
				}

				// skip the distro repos definition, if it has been already read
				_, ok := distrosRepoConfigs[distro]
				if ok {
					continue
				}

				configFile := filepath.Join(reposPath, fileEntry.Name())
				distroRepos, err := rpmmd.LoadRepositoriesFromFile(configFile)
				if err != nil {
					return nil, err
				}

				log.Println("Loaded repository configuration file:", configFile)

				distrosRepoConfigs[distro] = distroRepos
			}
		}
	}

	if len(distrosRepoConfigs) == 0 {
		return nil, &NoReposLoadedError{confPaths}
	}

	return distrosRepoConfigs, nil
}

// LoadRepositories loads distribution repositories from the given list of paths.
// If there are duplicate distro repositories definitions found in multiple paths, the first
// encounter is preferred. For this reason, the order of paths in the passed list should
// reflect the desired preference.
func LoadRepositories(confPaths []string, distro string) (map[string][]rpmmd.RepoConfig, error) {
	var repoConfigs map[string][]rpmmd.RepoConfig
	path := "/repositories/" + distro + ".json"

	for _, confPath := range confPaths {
		var err error
		repoConfigs, err = rpmmd.LoadRepositoriesFromFile(confPath + path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		// Found the distro repository configs in the current path
		if repoConfigs != nil {
			break
		}
	}

	if repoConfigs == nil {
		return nil, &NoReposLoadedError{confPaths}
	}

	return repoConfigs, nil
}
