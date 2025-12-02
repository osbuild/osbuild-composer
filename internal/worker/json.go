package worker

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"golang.org/x/exp/slices"
)

//
// JSON-serializable types for the jobqueue
//

type JobResult struct {
	JobError *clienterrors.Error `json:"job_error,omitempty"`
	Progress *JobProgress        `json:"progress,omitempty"`
}

type JobProgress struct {
	Message     string       `json:"message"`
	Done        int          `json:"done"`
	Total       int          `json:"total"`
	SubProgress *JobProgress `json:"sub_progress,omitempty"`
}

type OSBuildJob struct {
	Manifest manifest.OSBuildManifest `json:"manifest,omitempty"`

	// Index of the ManifestJobByIDResult instance in the job's dynamic arguments slice
	ManifestDynArgsIdx *int `json:"manifest_dyn_args_idx,omitempty"`

	// Index of the DepsolveJobResult instance in the job's dynamic arguments slice
	// This is used only for Koji composes, which need to have access to SBOMs produced
	// as part of the depsolve job, so that they can be uploaded to Koji.
	DepsolveDynArgsIdx *int `json:"depsolve_dyn_args_idx,omitempty"`

	Targets       []*target.Target `json:"targets,omitempty"`
	PipelineNames *PipelineNames   `json:"pipeline_names,omitempty"`

	// The ImageBootMode is just copied to the result by the worker, so that
	// the value can be accessed job which depend on it.
	// (string representation of distro.BootMode values)
	ImageBootMode string `json:"image_boot_mode,omitempty"`
}

// OsbuildExports returns a slice of osbuild pipeline names, which should be
// exported as part of running osbuild image build for the job. The pipeline
// names are gathered from the targets specified in the job.
func (j OSBuildJob) OsbuildExports() []string {
	exports := []string{}
	seenExports := map[string]bool{}
	for _, target := range j.Targets {
		exists := seenExports[target.OsbuildArtifact.ExportName]
		if !exists {
			seenExports[target.OsbuildArtifact.ExportName] = true
			exports = append(exports, target.OsbuildArtifact.ExportName)
		}
	}
	return exports
}

type OSBuildJobResult struct {
	Success       bool                   `json:"success"`
	OSBuildOutput *osbuild.Result        `json:"osbuild_output,omitempty"`
	TargetResults []*target.TargetResult `json:"target_results,omitempty"`
	UploadStatus  string                 `json:"upload_status"`
	PipelineNames *PipelineNames         `json:"pipeline_names,omitempty"`
	// Host OS of the worker which handled the job
	HostOS string `json:"host_os"`
	// Architecture of the worker which handled the job
	Arch string `json:"arch"`
	// Boot mode supported by the image
	// (string representation of distro.BootMode values)
	ImageBootMode string `json:"image_boot_mode,omitempty"`
	// Version of the osbuild binary used by the worker to build the image
	OSBuildVersion string `json:"osbuild_version,omitempty"`
	JobResult
}

// TargetErrors returns a slice of *clienterrors.Error gathered
// from the job result's target results. If there were no target errors
// then the returned slice will be empty.
func (j *OSBuildJobResult) TargetErrors() []*clienterrors.Error {
	targetErrors := []*clienterrors.Error{}

	for _, targetResult := range j.TargetResults {
		if targetResult.TargetError != nil {
			targetError := targetResult.TargetError
			// Add the target name to the error details, because the error reason
			// may not contain any information to determine the type of the target
			// which failed.
			targetErrors = append(targetErrors, clienterrors.New(targetError.ID, targetError.Reason, targetResult.Name))
		}
	}

	return targetErrors
}

// TargetResultsByName iterates over TargetResults attached to the Job result and
// returns a slice of Target results of the provided name (type). If there were no
// TargetResults of the desired type attached to the Job results, the returned
// slice will be empty.
func (j *OSBuildJobResult) TargetResultsByName(name target.TargetName) []*target.TargetResult {
	targetResults := []*target.TargetResult{}
	for _, targetResult := range j.TargetResults {
		if targetResult.Name == name {
			targetResults = append(targetResults, targetResult)
		}
	}
	return targetResults
}

// TargetResultsFilterByName iterates over TargetResults attached to the Job result and
// returns a slice of Target results excluding the provided names (types). If there were
// no TargetResults left after filtering, the returned slice will be empty.
func (j *OSBuildJobResult) TargetResultsFilterByName(excludeNames []target.TargetName) []*target.TargetResult {
	targetResults := []*target.TargetResult{}
	for _, targetResult := range j.TargetResults {
		if !slices.Contains(excludeNames, targetResult.Name) {
			targetResults = append(targetResults, targetResult)
		}
	}
	return targetResults
}

func (j *FileResolveJobResult) ResolutionErrors() []*clienterrors.Error {
	resolutionErrors := []*clienterrors.Error{}

	for _, result := range j.Results {
		if result.ResolutionError != nil {
			resolutionErrors = append(resolutionErrors, result.ResolutionError)
		}
	}

	return resolutionErrors
}

type KojiInitJob struct {
	Server  string `json:"server"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
}

type KojiInitJobResult struct {
	BuildID   uint64 `json:"build_id"`
	Token     string `json:"token"`
	KojiError string `json:"koji_error"`
	JobResult
}

type KojiFinalizeJob struct {
	Server  string `json:"server"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release"`
	// TODO: eventually deprecate and remove KojiFilenames, since the image filenames are now set in the KojiTargetResultOptions.
	KojiFilenames []string `json:"koji_filenames"`
	KojiDirectory string   `json:"koji_directory"`
	TaskID        uint64   `json:"task_id"` /* https://pagure.io/koji/issue/215 */
	StartTime     uint64   `json:"start_time"`
}

type KojiFinalizeJobResult struct {
	KojiError string `json:"koji_error"`
	JobResult
}

// PipelineNames is used to provide two pieces of information related to a job:
// 1. A categorization of each pipeline into one of two groups
// 2. A pipeline ordering when the lists are concatenated: build -> os
type PipelineNames struct {
	Build   []string `json:"build"`
	Payload []string `json:"payload"`
}

// Returns a concatenated list of the pipeline names
func (pn *PipelineNames) All() []string {
	return append(pn.Build, pn.Payload...)
}

// DepsolveJob defines the parameters of one or more depsolve jobs.  Each named
// list of package sets defines a separate job.  Lists with multiple package
// sets are depsolved in a chain, combining the results of sequential depsolves
// into a single PackageList in the result.  Each PackageSet defines the
// repositories it will be depsolved against.
type DepsolveJob struct {
	PackageSets      map[string][]rpmmd.PackageSet `json:"grouped_package_sets"`
	ModulePlatformID string                        `json:"module_platform_id"`
	Arch             string                        `json:"arch"`
	Releasever       string                        `json:"releasever"`

	// NB: for now, the worker supports only a single SBOM type, but keep the options
	// open for the future by passing the actual type and not just bool.
	SbomType sbom.StandardType `json:"sbom_type,omitempty"`
}

// SbomDoc represents a single SBOM document result.
type SbomDoc struct {
	DocType  sbom.StandardType `json:"type"`
	Document json.RawMessage   `json:"document"`
}

type DepsolvedPackageChecksum struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func (c *DepsolvedPackageChecksum) UnmarshalJSON(data []byte) error {
	var rawChecksum interface{}
	err := json.Unmarshal(data, &rawChecksum)
	if err != nil {
		return err
	}
	switch rawChecksum := rawChecksum.(type) {
	case nil:
		// explicit "checksum": null, leave it as nil. This should not happen, but it is better to be safe.
	case string:
		checksumParts := strings.Split(rawChecksum, ":")
		if len(checksumParts) != 2 {
			return fmt.Errorf("invalid checksum format: %q", rawChecksum)
		}
		*c = DepsolvedPackageChecksum{
			Type:  checksumParts[0],
			Value: checksumParts[1],
		}
	case map[string]interface{}:
		if _, ok := rawChecksum["type"]; !ok {
			return fmt.Errorf("checksum type is required, got %+v", rawChecksum)
		}
		if _, ok := rawChecksum["value"]; !ok {
			return fmt.Errorf("checksum value is required, got %+v", rawChecksum)
		}
		*c = DepsolvedPackageChecksum{
			Type:  rawChecksum["type"].(string),
			Value: rawChecksum["value"].(string),
		}
	default:
		return fmt.Errorf("unsupported checksum type: %T", rawChecksum)
	}

	return nil
}

func (c *DepsolvedPackageChecksum) MarshalJSON() ([]byte, error) {
	if c == nil {
		return json.Marshal(nil)
	}
	// For backward compatibility reason, keep the string representation of the checksum.
	// TODO: switch to the struct after a few releases. (added on 2025-10-08)
	return json.Marshal(fmt.Sprintf("%s:%s", c.Type, c.Value))
}

type DepsolvedPackageRelDep struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship,omitempty"`
	Version      string `json:"version,omitempty"`
}

type DepsolvedPackageRelDepList []DepsolvedPackageRelDep

func (d DepsolvedPackageRelDepList) ToRPMMDList() rpmmd.RelDepList {
	results := make(rpmmd.RelDepList, len(d))
	for i, relDep := range d {
		results[i] = rpmmd.RelDep(relDep)
	}
	return results
}

func DepsolvedPackageRelDepListFromRPMMDList(relDeps rpmmd.RelDepList) DepsolvedPackageRelDepList {
	results := make(DepsolvedPackageRelDepList, len(relDeps))
	for i, relDep := range relDeps {
		results[i] = DepsolvedPackageRelDep(relDep)
	}
	return results
}

// DepsolvedPackage is the DTO for rpmmd.Package.
type DepsolvedPackage struct {
	Name    string `json:"name"`
	Epoch   uint   `json:"epoch"`
	Version string `json:"version,omitempty"`
	Release string `json:"release,omitempty"`
	Arch    string `json:"arch,omitempty"`

	Group string `json:"group,omitempty"`

	DownloadSize uint64 `json:"download_size,omitempty"`
	InstallSize  uint64 `json:"install_size,omitempty"`

	License   string `json:"license,omitempty"`
	SourceRpm string `json:"source_rpm,omitempty"`

	BuildTime *time.Time `json:"build_time,omitempty"`
	Packager  string     `json:"packager,omitempty"`
	Vendor    string     `json:"vendor,omitempty"`

	URL string `json:"url,omitempty"`

	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`

	Provides        DepsolvedPackageRelDepList `json:"provides,omitempty"`
	Requires        DepsolvedPackageRelDepList `json:"requires,omitempty"`
	RequiresPre     DepsolvedPackageRelDepList `json:"requires_pre,omitempty"`
	Conflicts       DepsolvedPackageRelDepList `json:"conflicts,omitempty"`
	Obsoletes       DepsolvedPackageRelDepList `json:"obsoletes,omitempty"`
	RegularRequires DepsolvedPackageRelDepList `json:"regular_requires,omitempty"`

	Recommends  DepsolvedPackageRelDepList `json:"recommends,omitempty"`
	Suggests    DepsolvedPackageRelDepList `json:"suggests,omitempty"`
	Enhances    DepsolvedPackageRelDepList `json:"enhances,omitempty"`
	Supplements DepsolvedPackageRelDepList `json:"supplements,omitempty"`

	Files []string `json:"files,omitempty"`

	BaseURL         string   `json:"base_url,omitempty"`
	Location        string   `json:"location,omitempty"`
	RemoteLocations []string `json:"remote_locations,omitempty"`

	Checksum       *DepsolvedPackageChecksum `json:"checksum,omitempty"`
	HeaderChecksum *DepsolvedPackageChecksum `json:"header_checksum,omitempty"`

	RepoID string `json:"repo_id,omitempty"`

	Reason string `json:"reason,omitempty"`

	Secrets   string `json:"secrets,omitempty"`
	CheckGPG  bool   `json:"check_gpg,omitempty"`
	IgnoreSSL bool   `json:"ignore_ssl,omitempty"`
}

// UnmarshalJSON is used to unmarshal the DepsolvedPackage from JSON.
// This handles the case when old composer-worker and new composer-worker-server
// are used.
func (d *DepsolvedPackage) UnmarshalJSON(data []byte) error {
	type aliasType DepsolvedPackage
	type compatType struct {
		aliasType

		// TODO: remove this after a few releases (added on 2025-10-08)
		// The type was changed to rpmmd.Package, but the fields were kept for backwards compatibility.
		/* Legacy type before the rpmmd RPM package consolidation

		type DepsolvedPackage struct {
			Name           string `json:"name"`
			Epoch          uint   `json:"epoch"`
			Version        string `json:"version,omitempty"`
			Release        string `json:"release,omitempty"`
			Arch           string `json:"arch,omitempty"`
			RemoteLocation string `json:"remote_location,omitempty"`
			Checksum       string `json:"checksum,omitempty"`
			Secrets        string `json:"secrets,omitempty"`
			CheckGPG       bool   `json:"check_gpg,omitempty"`
			IgnoreSSL      bool   `json:"ignore_ssl,omitempty"`

			Path   string `json:"path,omitempty"`
			RepoID string `json:"repo_id,omitempty"`
		}

		*/
		// Path is now called Location in rpmmd.Package.
		Path string `json:"path,omitempty"` // obsolete
		// RemoteLocation is now called RemoteLocations in rpmmd.Package and is a slice.
		RemoteLocation string `json:"remote_location,omitempty"` // obsolete
	}

	var compat compatType
	err := json.Unmarshal(data, &compat)
	if err != nil {
		return err
	}

	// Handle Path vs. Location.
	if compat.aliasType.Location == "" && compat.Path != "" {
		compat.aliasType.Location = compat.Path
	}
	// Handle RemoteLocation vs. RemoteLocations.
	if compat.aliasType.RemoteLocations == nil && compat.RemoteLocation != "" {
		compat.aliasType.RemoteLocations = []string{compat.RemoteLocation}
	}

	*d = DepsolvedPackage(compat.aliasType)

	return nil
}

// MarshalJSON is used to marshal the DepsolvedPackage to JSON.
// This handles the case when old composer-worker-server and new composer-worker
// are used.
func (d DepsolvedPackage) MarshalJSON() ([]byte, error) {
	type aliasType DepsolvedPackage
	type compatType struct {
		aliasType

		// TODO: remove this after a few releases (added on 2025-10-08)
		// The type was changed to rpmmd.Package, but the fields were kept for backwards compatibility.
		/* Legacy type before the rpmmd RPM package consolidation

		type DepsolvedPackage struct {
			Name           string `json:"name"`
			Epoch          uint   `json:"epoch"`
			Version        string `json:"version,omitempty"`
			Release        string `json:"release,omitempty"`
			Arch           string `json:"arch,omitempty"`
			RemoteLocation string `json:"remote_location,omitempty"`
			Checksum       string `json:"checksum,omitempty"`
			Secrets        string `json:"secrets,omitempty"`
			CheckGPG       bool   `json:"check_gpg,omitempty"`
			IgnoreSSL      bool   `json:"ignore_ssl,omitempty"`

			Path   string `json:"path,omitempty"`
			RepoID string `json:"repo_id,omitempty"`
		}

		*/
		// Path is now called Location in rpmmd.Package.
		Path string `json:"path,omitempty"` // obsolete
		// RemoteLocation is now called RemoteLocations in rpmmd.Package and is a slice.
		RemoteLocation string `json:"remote_location,omitempty"` // obsolete
	}

	var compat compatType
	compat.aliasType = aliasType(d)

	// Handle Path vs. Location.
	compat.Path = compat.aliasType.Location

	// Handle RemoteLocation vs. RemoteLocations.
	if len(compat.aliasType.RemoteLocations) > 0 {
		compat.RemoteLocation = compat.aliasType.RemoteLocations[0]
	}

	return json.Marshal(compat)
}

func (d DepsolvedPackage) ToRPMMD() rpmmd.Package {
	p := rpmmd.Package{
		Name:    d.Name,
		Epoch:   d.Epoch,
		Version: d.Version,
		Release: d.Release,
		Arch:    d.Arch,

		Group: d.Group,

		DownloadSize: d.DownloadSize,
		InstallSize:  d.InstallSize,

		License: d.License,

		SourceRpm: d.SourceRpm,

		Packager: d.Packager,
		Vendor:   d.Vendor,

		URL: d.URL,

		Summary:     d.Summary,
		Description: d.Description,

		Provides:        d.Provides.ToRPMMDList(),
		Requires:        d.Requires.ToRPMMDList(),
		RequiresPre:     d.RequiresPre.ToRPMMDList(),
		Conflicts:       d.Conflicts.ToRPMMDList(),
		Obsoletes:       d.Obsoletes.ToRPMMDList(),
		RegularRequires: d.RegularRequires.ToRPMMDList(),

		Recommends:  d.Recommends.ToRPMMDList(),
		Suggests:    d.Suggests.ToRPMMDList(),
		Enhances:    d.Enhances.ToRPMMDList(),
		Supplements: d.Supplements.ToRPMMDList(),

		Files: d.Files,

		BaseURL:         d.BaseURL,
		Location:        d.Location,
		RemoteLocations: d.RemoteLocations,

		RepoID: d.RepoID,

		Reason: d.Reason,

		Secrets:   d.Secrets,
		CheckGPG:  d.CheckGPG,
		IgnoreSSL: d.IgnoreSSL,
	}

	if d.BuildTime != nil {
		p.BuildTime = *d.BuildTime
	}

	if d.Checksum != nil {
		p.Checksum = rpmmd.Checksum(*d.Checksum)
	}

	if d.HeaderChecksum != nil {
		p.HeaderChecksum = rpmmd.Checksum(*d.HeaderChecksum)
	}

	return p
}

type DepsolvedPackageList []DepsolvedPackage

func (d DepsolvedPackageList) ToRPMMDList() rpmmd.PackageList {
	results := make(rpmmd.PackageList, len(d))
	for i, pkg := range d {
		results[i] = pkg.ToRPMMD()
	}
	return results
}

func DepsolvedPackageFromRPMMD(pkg rpmmd.Package) DepsolvedPackage {
	return DepsolvedPackage{
		Name:    pkg.Name,
		Epoch:   pkg.Epoch,
		Version: pkg.Version,
		Release: pkg.Release,
		Arch:    pkg.Arch,

		Group: pkg.Group,

		DownloadSize: pkg.DownloadSize,
		InstallSize:  pkg.InstallSize,

		License: pkg.License,

		SourceRpm: pkg.SourceRpm,

		BuildTime: &pkg.BuildTime,
		Packager:  pkg.Packager,
		Vendor:    pkg.Vendor,

		URL: pkg.URL,

		Summary:     pkg.Summary,
		Description: pkg.Description,

		Provides:        DepsolvedPackageRelDepListFromRPMMDList(pkg.Provides),
		Requires:        DepsolvedPackageRelDepListFromRPMMDList(pkg.Requires),
		RequiresPre:     DepsolvedPackageRelDepListFromRPMMDList(pkg.RequiresPre),
		Conflicts:       DepsolvedPackageRelDepListFromRPMMDList(pkg.Conflicts),
		Obsoletes:       DepsolvedPackageRelDepListFromRPMMDList(pkg.Obsoletes),
		RegularRequires: DepsolvedPackageRelDepListFromRPMMDList(pkg.RegularRequires),

		Recommends:  DepsolvedPackageRelDepListFromRPMMDList(pkg.Recommends),
		Suggests:    DepsolvedPackageRelDepListFromRPMMDList(pkg.Suggests),
		Enhances:    DepsolvedPackageRelDepListFromRPMMDList(pkg.Enhances),
		Supplements: DepsolvedPackageRelDepListFromRPMMDList(pkg.Supplements),

		Files: pkg.Files,

		BaseURL:         pkg.BaseURL,
		Location:        pkg.Location,
		RemoteLocations: pkg.RemoteLocations,

		Checksum:       common.ToPtr(DepsolvedPackageChecksum(pkg.Checksum)),
		HeaderChecksum: common.ToPtr(DepsolvedPackageChecksum(pkg.HeaderChecksum)),

		RepoID: pkg.RepoID,

		Reason: pkg.Reason,

		Secrets:   pkg.Secrets,
		CheckGPG:  pkg.CheckGPG,
		IgnoreSSL: pkg.IgnoreSSL,
	}
}

func DepsolvedPackageListFromRPMMDList(pkgs rpmmd.PackageList) DepsolvedPackageList {
	results := make(DepsolvedPackageList, len(pkgs))
	for i, pkg := range pkgs {
		results[i] = DepsolvedPackageFromRPMMD(pkg)
	}
	return results
}

type DepsolveJobResult struct {
	PackageSpecs map[string]DepsolvedPackageList `json:"package_specs"`
	SbomDocs     map[string]SbomDoc              `json:"sbom_docs,omitempty"`
	RepoConfigs  map[string][]rpmmd.RepoConfig   `json:"repo_configs"`
	JobResult
}

// SearchPackagesJob defines the parameters for a dnf metadata search
// It will search the included repositories for packages matching the
// package strings
// Package names support globs using '*' and will search for a substring
// match if '*foopkg*' is used.
type SearchPackagesJob struct {
	Packages         []string           `json:"packages"`
	Repositories     []rpmmd.RepoConfig `json:"repos"`
	ModulePlatformID string             `json:"module_platform_id"`
	Arch             string             `json:"arch"`
	Releasever       string             `json:"releasever"`
}

// SearchPackagesJobResult returns the details of the search packages
type SearchPackagesJobResult struct {
	Packages rpmmd.PackageList `json:"packages"`
	JobResult
}

type ManifestJobByID struct{}

// OSBuildComposerDepModule contains information about a module used by
// osbuild-composer which could affect the manifest content.
type OSBuildComposerDepModule struct {
	Path    string                    `json:"path"`
	Version string                    `json:"version"`
	Replace *OSBuildComposerDepModule `json:"replace,omitempty"`
}

// ComposerDepModuleFromDebugModule converts a debug.Module instance
// to an OSBuildComposerDepModule instance.
func ComposerDepModuleFromDebugModule(module *debug.Module) *OSBuildComposerDepModule {
	if module == nil {
		return nil
	}
	depModule := &OSBuildComposerDepModule{
		Path:    module.Path,
		Version: module.Version,
	}
	if module.Replace != nil {
		depModule.Replace = &OSBuildComposerDepModule{
			Path:    module.Replace.Path,
			Version: module.Replace.Version,
		}
	}
	return depModule
}

// ManifestInfo contains information about the environment in which
// the manifest was produced and which could affect its content.
type ManifestInfo struct {
	OSBuildComposerVersion string `json:"osbuild_composer_version"`
	// List of relevant modules used by osbuild-composer which
	// could affect the manifest content.
	OSBuildComposerDeps []*OSBuildComposerDepModule `json:"osbuild_composer_deps,omitempty"`

	// Ordered list of pipeline names in the manifest, separated into build and
	// payload pipelines. These are parsed from the manifest itself and copied
	// to the osbuild job result so it can properly order the osbuild job log
	// by pipeline execution order.
	PipelineNames *PipelineNames `json:"pipeline_names,omitempty"`
}

type ManifestJobByIDResult struct {
	Manifest     manifest.OSBuildManifest `json:"data,omitempty"`
	ManifestInfo ManifestInfo             `json:"info,omitempty"`
	Error        string                   `json:"error"`
	JobResult
}

type ContainerSpec struct {
	Source    string `json:"source"`
	Name      string `json:"name"`
	TLSVerify *bool  `json:"tls-verify,omitempty"`

	ImageID    string `json:"image_id"`
	Digest     string `json:"digest"`
	ListDigest string `json:"list-digest,omitempty"`
}

type ContainerResolveJob struct {
	Arch  string          `json:"arch"`
	Specs []ContainerSpec `json:"specs"`
}

type ContainerResolveJobResult struct {
	Specs []ContainerSpec `json:"specs"`

	JobResult
}

type FileResolveJob struct {
	URLs []string `json:"urls"`
}

type FileResolveJobResultItem struct {
	URL             string              `json:"url"`
	Content         []byte              `json:"content"`
	ResolutionError *clienterrors.Error `json:"target_error,omitempty"`
}

type FileResolveJobResult struct {
	Success bool                       `json:"success"`
	Results []FileResolveJobResultItem `json:"results"`
	JobResult
}

type OSTreeResolveSpec struct {
	URL  string `json:"url"`
	Ref  string `json:"ref"`
	RHSM bool   `json:"rhsm"`
}

type OSTreeResolveJob struct {
	Specs []OSTreeResolveSpec `json:"ostree_resolve_specs"`
}

type OSTreeResolveResultSpec struct {
	URL      string `json:"url"`
	Ref      string `json:"ref"`
	Checksum string `json:"checksum"`
	RHSM     bool   `json:"bool"` // NOTE: kept for backwards compatibility; remove after a few releases
	Secrets  string `json:"secrets"`
}

type OSTreeResolveJobResult struct {
	Specs []OSTreeResolveResultSpec `json:"ostree_resolve_result_specs"`

	JobResult
}

type AWSEC2ShareJob struct {
	Ami               string   `json:"ami"`
	Region            string   `json:"region"`
	ShareWithAccounts []string `json:"shareWithAccounts"`
}

type AWSEC2ShareJobResult struct {
	JobResult

	Ami    string `json:"ami"`
	Region string `json:"region"`
}

type AWSEC2CopyJob struct {
	Ami          string `json:"ami"`
	SourceRegion string `json:"source_region"`
	TargetRegion string `json:"target_region"`
	TargetName   string `json:"target_name"`
}

type AWSEC2CopyJobResult struct {
	JobResult

	Ami    string `json:"ami"`
	Region string `json:"region"`
}

// ImageBuilderManifestJob generates a manifest from a build request using
// image-builder-cli. Includes resolving all content types.
type ImageBuilderManifestJob struct {
	// Arguments to the image-builder command line
	Args ImageBuilderArgs

	// Extra environment variables
	ExtraEnv []string
}

// ImageBuilderManifestJobResult is an alias to [ManifestJobByIDResult].
type ImageBuilderManifestJobResult = ManifestJobByIDResult
