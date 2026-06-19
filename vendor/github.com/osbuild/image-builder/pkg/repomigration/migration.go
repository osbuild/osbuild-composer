// Package repomigration holds functions from the blueprint library that we
// need to define here temporarily while we update import paths from
// osbuild/images to osbuild/image-builder.
//
// What we need to accomplish is:
// 1. Change the package name for image-builder
// 2. Update the import paths for image-builder
// 3. Update the import paths for blueprint
//
// However, changing a type from images/package.Type to
// image-builder/package.Type and trying to pass it to a blueprint function
// causes an incompatible type error. Therefore, blueprint functions used by
// image-builder will be copied here and then removed when everything is ready.
package repomigration

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/pathpolicy"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckMountpointsPolicy(mountpoints []blueprint.FilesystemCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		err := mountpointAllowList.Check(m.Mountpoint)
		if err != nil {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	return nil
}

// CheckFileCustomizationsPolicy checks if the given File customizations are allowed by the path policy.
// If any of the customizations are not allowed by the path policy, an error is returned. Otherwise, nil is returned.
func CheckFileCustomizationsPolicy(files []blueprint.FileCustomization, pathPolicy *pathpolicy.PathPolicies) error {
	var invalidPaths []string
	for _, file := range files {
		if err := pathPolicy.Check(file.Path); err != nil {
			invalidPaths = append(invalidPaths, file.Path)
		}
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("the following custom files are not allowed: %+q", invalidPaths)
	}

	return nil
}

// CheckDirectoryCustomizationsPolicy checks if the given Directory customizations are allowed by the path policy.
// If any of the customizations are not allowed by the path policy, an error is returned. Otherwise, nil is returned.
func CheckDirectoryCustomizationsPolicy(dirs []blueprint.DirectoryCustomization, pathPolicy *pathpolicy.PathPolicies) error {
	var invalidPaths []string
	for _, dir := range dirs {
		if err := pathPolicy.Check(dir.Path); err != nil {
			invalidPaths = append(invalidPaths, dir.Path)
		}
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("the following custom directories are not allowed: %+q", invalidPaths)
	}

	return nil
}

// CheckDiskMountpointsPolicy checks if the mountpoints under a [DiskCustomization] are allowed by the policy.
func CheckDiskMountpointsPolicy(partitioning *blueprint.DiskCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	if partitioning == nil {
		return nil
	}

	// collect all mountpoints
	var mountpoints []string
	for _, part := range partitioning.Partitions {
		if part.Mountpoint != "" {
			mountpoints = append(mountpoints, part.Mountpoint)
		}
		for _, lv := range part.LogicalVolumes {
			if lv.Mountpoint != "" {
				mountpoints = append(mountpoints, lv.Mountpoint)
			}
		}
		for _, subvol := range part.Subvolumes {
			mountpoints = append(mountpoints, subvol.Mountpoint)
		}
	}

	var errs []error
	for _, mp := range mountpoints {
		if err := mountpointAllowList.Check(mp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while setting up custom mountpoints:\n%w", errors.Join(errs...))
	}

	return nil
}

// FileCustomizationsToFsNodeFiles converts a slice of FileCustomization to a slice of *fsnode.File
func FileCustomizationsToFsNodeFiles(files []blueprint.FileCustomization) ([]*fsnode.File, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var fsFiles []*fsnode.File
	var errors []error
	for _, file := range files {
		fsFile, err := ToFsNodeFile(file)
		if err != nil {
			errors = append(errors, err)
		}

		fsFiles = append(fsFiles, fsFile)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("invalid file customizations: %v", errors)
	}

	return fsFiles, nil
}

// ToFsNodeFile converts the FileCustomization to an fsnode.File
func ToFsNodeFile(f blueprint.FileCustomization) (*fsnode.File, error) {
	if f.Data != "" && f.URI != "" {
		return nil, fmt.Errorf("cannot specify both data %q and URI %q", f.Data, f.URI)
	}

	var data []byte
	if f.Data != "" {
		data = []byte(f.Data)
	}

	var mode *os.FileMode
	if f.Mode != "" {
		err := validateModeString(f.Mode)
		if err != nil {
			return nil, err
		}
		modeNum, err := strconv.ParseUint(f.Mode, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mode %s: %v", f.Mode, err)
		}
		mode = common.ToPtr(os.FileMode(modeNum))
	}

	if f.URI != "" {
		return fsnode.NewFileForURI(f.Path, mode, f.User, f.Group, f.URI)
	}
	return fsnode.NewFile(f.Path, mode, f.User, f.Group, data)
}

// validateModeString checks that the given string is a valid mode octal number
func validateModeString(mode string) error {
	// Check that the mode string matches the octal format regular expression.
	// The leading is optional.
	if regexp.MustCompile(`^[0]{0,1}[0-7]{3}$`).MatchString(mode) {
		return nil
	}
	return fmt.Errorf("invalid mode %s: must be an octal number", mode)
}

// ToFsNodeDirectory converts the DirectoryCustomization to an fsnode.Directory
func ToFsNodeDirectory(d blueprint.DirectoryCustomization) (*fsnode.Directory, error) {
	var mode *os.FileMode
	if d.Mode != "" {
		err := validateModeString(d.Mode)
		if err != nil {
			return nil, err
		}
		modeNum, err := strconv.ParseUint(d.Mode, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mode %s: %v", d.Mode, err)
		}
		// modeNum is parsed as an unsigned 32 bit int
		/* #nosec G115 */
		mode = common.ToPtr(os.FileMode(modeNum))
	}

	return fsnode.NewDirectory(d.Path, mode, d.User, d.Group, d.EnsureParents)
}

// DirectoryCustomizationsToFsNodeDirectories converts a slice of DirectoryCustomizations
// to a slice of fsnode.Directories
func DirectoryCustomizationsToFsNodeDirectories(dirs []blueprint.DirectoryCustomization) ([]*fsnode.Directory, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	var fsDirs []*fsnode.Directory
	var errors []error
	for _, dir := range dirs {
		fsDir, err := ToFsNodeDirectory(dir)
		if err != nil {
			errors = append(errors, err)
		}
		fsDirs = append(fsDirs, fsDir)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("invalid directory customizations: %v", errors)
	}

	return fsDirs, nil
}

func RepoCustomizationsToRepoConfigAndGPGKeyFiles(repos []blueprint.RepositoryCustomization) (map[string][]rpmmd.RepoConfig, []*fsnode.File, error) {
	if len(repos) == 0 {
		return nil, nil, nil
	}

	repoMap := make(map[string][]rpmmd.RepoConfig, len(repos))
	var gpgKeyFiles []*fsnode.File
	for _, repo := range repos {
		filename := getFilename(repo)
		convertedRepo := customRepoToRepoConfig(repo)

		// convert any inline gpgkeys to fsnode.File and
		// replace the gpgkey with the file path
		for idx, gpgkey := range repo.GPGKeys {
			if _, ok := url.ParseRequestURI(gpgkey); ok != nil {
				// create the file path
				path := fmt.Sprintf("/etc/pki/rpm-gpg/RPM-GPG-KEY-%s-%d", repo.Id, idx)
				// replace the gpgkey with the file path
				convertedRepo.GPGKeys[idx] = fmt.Sprintf("file://%s", path)
				// create the fsnode for the gpgkey keyFile
				keyFile, err := fsnode.NewFile(path, nil, nil, nil, []byte(gpgkey))
				if err != nil {
					return nil, nil, err
				}
				gpgKeyFiles = append(gpgKeyFiles, keyFile)
			}
		}

		repoMap[filename] = append(repoMap[filename], convertedRepo)
	}

	return repoMap, gpgKeyFiles, nil
}

func getFilename(rc blueprint.RepositoryCustomization) string {
	if rc.Filename == "" {
		return fmt.Sprintf("%s.repo", rc.Id)
	}
	if !strings.HasSuffix(rc.Filename, ".repo") {
		return fmt.Sprintf("%s.repo", rc.Filename)
	}
	return rc.Filename
}

func customRepoToRepoConfig(repo blueprint.RepositoryCustomization) rpmmd.RepoConfig {
	urls := make([]string, len(repo.BaseURLs))
	copy(urls, repo.BaseURLs)

	keys := make([]string, len(repo.GPGKeys))
	copy(keys, repo.GPGKeys)

	repoConfig := rpmmd.RepoConfig{
		Id:             repo.Id,
		BaseURLs:       urls,
		GPGKeys:        keys,
		Name:           repo.Name,
		Metalink:       repo.Metalink,
		MirrorList:     repo.Mirrorlist,
		CheckGPG:       repo.GPGCheck,
		CheckRepoGPG:   repo.RepoGPGCheck,
		Priority:       repo.Priority,
		ModuleHotfixes: repo.ModuleHotfixes,
		Enabled:        repo.Enabled,
	}

	if repo.SSLVerify != nil {
		repoConfig.IgnoreSSL = common.ToPtr(!*repo.SSLVerify)
	}

	return repoConfig
}

func RepoCustomizationsInstallFromOnly(repos []blueprint.RepositoryCustomization) []rpmmd.RepoConfig {
	var res []rpmmd.RepoConfig
	for _, repo := range repos {
		if !repo.InstallFrom {
			continue
		}
		res = append(res, customRepoToRepoConfig(repo))
	}
	return res
}
