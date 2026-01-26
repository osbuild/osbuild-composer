package reporegistry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/distroidparser"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/rpmmd"
)

// LoadAllRepositories loads all repositories for given distros from the given list of paths.
// Behavior is the same as with the LoadRepositories() method.
func LoadAllRepositories(confPaths []string, confFSes []fs.FS) (rpmmd.DistrosRepoConfigs, error) {
	var mergedFSes []fs.FS

	for _, confPath := range confPaths {
		mergedFSes = append(mergedFSes, os.DirFS(confPath))
	}
	mergedFSes = append(mergedFSes, confFSes...)

	distrosRepoConfigs, err := loadAllRepositoriesFromFS(mergedFSes)
	if err != nil {
		return nil, fmt.Errorf("failed to load repositories: %w", err)
	}
	if len(distrosRepoConfigs) == 0 {
		return nil, &NoReposLoadedError{confPaths, confFSes}
	}
	return distrosRepoConfigs, nil
}

func loadAllRepositoriesFromFS(confPaths []fs.FS) (rpmmd.DistrosRepoConfigs, error) {
	distrosRepoConfigs := rpmmd.DistrosRepoConfigs{}

	for _, confPath := range confPaths {
		fileEntries, err := fs.ReadDir(confPath, ".")
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
			if strings.HasSuffix(fileEntry.Name(), ".json") || strings.HasSuffix(fileEntry.Name(), ".yaml") {
				distroIDStr := strings.TrimSuffix(fileEntry.Name(), ".json")
				distroIDStr = strings.TrimSuffix(distroIDStr, ".yaml")

				// compatibility layer to support old repository definition filenames
				// without a dot to separate major and minor release versions
				distro, err := distroidparser.DefaultParser.Standardize(distroIDStr)
				if err != nil {
					olog.Printf("WARNING: failed to parse distro ID string, using it as is: %v", err)
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

				configFile, err := confPath.Open(fileEntry.Name())
				if err != nil {
					return nil, err
				}
				distroRepos, err := rpmmd.LoadRepositoriesFromReader(configFile)
				if err != nil {
					return nil, err
				}

				olog.Printf("Loaded repository configuration file: %s", fileEntry.Name())

				distrosRepoConfigs[distro] = distroRepos
			}
		}
	}

	return distrosRepoConfigs, nil
}

// LoadRepositories loads distribution repositories from the given list of paths.
// If there are duplicate distro repositories definitions found in multiple paths, the first
// encounter is preferred. For this reason, the order of paths in the passed list should
// reflect the desired preference. Both json and yaml repository files can be used to load
// from. When a json file is encountered it takes precedence over a yaml file under the
// same distro name.
//
// Note that the confPaths must point directly to the directory with
// the json and yaml repo files.
func LoadRepositories(confPaths []string, distro string) (map[string][]rpmmd.RepoConfig, error) {
	var repoConfigs map[string][]rpmmd.RepoConfig

	for _, confPath := range confPaths {
		var err error
		paths := []string{distro + ".json", distro + ".yaml"}
		for _, path := range paths {
			repoConfigs, err = rpmmd.LoadRepositoriesFromFile(filepath.Join(confPath, path))
			if os.IsNotExist(err) {
				continue
			} else if err != nil {
				return nil, err
			} else {
				break
			}
		}

		// Found the distro repository configs in the current path
		if repoConfigs != nil {
			break
		}
	}

	if repoConfigs == nil {
		return nil, &NoReposLoadedError{confPaths, nil}
	}

	return repoConfigs, nil
}
