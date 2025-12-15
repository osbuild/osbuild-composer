package rpmmd

import (
	"fmt"
	"time"
)

// RelDep represents an RPM dependency with a name, an optional relationship
// operator and an optional version.
type RelDep struct {
	// Name is the name of the dependency, e.g. "openssl-libs".
	Name string
	// Relationship to the version, e.g. "=", ">", ">=", etc. (optional)
	Relationship string
	// Version of the dependency, e.g. "3.0.1", etc. (optional)
	Version string
}

type RelDepList []RelDep

// Checksum represents a checksum with its type
type Checksum struct {
	// Type is the type of checksum, e.g. "sha256", "sha512", "md5", etc.
	Type string
	// Value is the checksum value, e.g. "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef" for sha256.
	Value string
}

func (c Checksum) String() string {
	return fmt.Sprintf("%s:%s", c.Type, c.Value)
}

// RPM package representation
//
// Based on libdnf5: https://github.com/rpm-software-management/dnf5/blob/main/include/libdnf5/rpm/package.hpp
// DNF4 version: https://github.com/rpm-software-management/libdnf/blob/dnf-4-master/libdnf/hy-package.h
//
// Some fields that are not relevant for us, or that can be deduced from existing fields, are omitted, specifically:
// - Various EVR or NEVRA getter methods, which would use existing fields
// - get_source_name() - RPM package source package name
// - get_debugsource_name() - RPM package debugsource package name
// - get_debuginfo_name_of_source() - RPM package debuginfo package name for the source package
// - get_debuginfo_name() - RPM package debuginfo package name
// - get_prereq_ignoreinst()
// - get_depends() - RPM package dependencies (requires + enhances + suggests + supplements + recommends)
// - get_changelogs()
// - get_hdr_end()
// - get_media_number()
// - get_package_path() - Path to the RPM package on the local file system
// - is_available_locally()
// - is_installed()
// - is_excluded()
// - get_from_repo_id() -  For an installed package, return id of repo from the package was installed
// - get_install_time()
// - get_rpmdbid()
type Package struct {
	Name    string
	Epoch   uint
	Version string
	Release string
	Arch    string

	// RPM package Group
	Group string

	// File size of the RPM package
	DownloadSize uint64
	// Size the RPM package should occupy after installing on disk
	// NB: The actual size on disk may vary based on block size and filesystem overhead.
	InstallSize uint64

	License string

	// RPM package source package filename
	SourceRpm string

	BuildTime time.Time
	Packager  string
	Vendor    string

	// RPM package URL (project home address)
	URL string

	Summary     string
	Description string

	// Regular dependencies
	Provides        RelDepList
	Requires        RelDepList // RegularRequires + PreRequires
	RequiresPre     RelDepList
	Conflicts       RelDepList
	Obsoletes       RelDepList
	RegularRequires RelDepList

	// Weak dependencies
	Recommends  RelDepList
	Suggests    RelDepList
	Enhances    RelDepList
	Supplements RelDepList

	// List of files and directories the RPM package contains
	Files []string

	// Repodata
	// RPM package relative path/location from repodata
	Location string
	// RPM package remote location where the package can be download from
	RemoteLocations []string

	// Checksum object representing RPM package checksum and its type
	Checksum Checksum
	// Checksum object representing RPM package header checksum and its type.
	HeaderChecksum Checksum

	// Repository ID this package belongs to
	// XXX: We should should eventually hold a reference to the RepoConfig
	RepoID string

	// Resolved reason why a package was / would be installed.
	Reason string

	// Convenience values coming from the respective repository config
	Secrets   string
	CheckGPG  bool
	IgnoreSSL bool
}

// EVRA returns the package's Epoch:Version-Release.Arch string.
// If the package Epoch is 0, it is omitted and only Version-Release.Arch is returned.
func (p Package) EVRA() string {
	if p.Epoch == 0 {
		return fmt.Sprintf("%s-%s.%s", p.Version, p.Release, p.Arch)
	}
	return fmt.Sprintf("%d:%s-%s.%s", p.Epoch, p.Version, p.Release, p.Arch)
}

// NVR returns the package's Name-Version-Release string.
func (p Package) NVR() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Version, p.Release)
}

type PackageList []Package

func (pl PackageList) Package(packageName string) (*Package, error) {
	for _, pkg := range pl {
		if pkg.Name == packageName {
			return &pkg, nil
		}
	}
	return nil, fmt.Errorf("package %q not found in the Package list", packageName)
}

// The inputs to depsolve, a set of packages to include and a set of packages
// to exclude. The Repositories are used when depsolving this package set in
// addition to the base repositories.
type PackageSet struct {
	Include         []string
	Exclude         []string
	EnabledModules  []string
	Repositories    []RepoConfig
	InstallWeakDeps bool
}

// Append the Include and Exclude package list from another PackageSet and
// return the result.
func (ps PackageSet) Append(other PackageSet) PackageSet {
	ps.Include = append(ps.Include, other.Include...)
	ps.Exclude = append(ps.Exclude, other.Exclude...)
	ps.EnabledModules = append(ps.EnabledModules, other.EnabledModules...)
	return ps
}
