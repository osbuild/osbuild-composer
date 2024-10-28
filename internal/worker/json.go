package worker

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"golang.org/x/exp/slices"
)

//
// JSON-serializable types for the jobqueue
//

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

type JobResult struct {
	JobError *clienterrors.Error `json:"job_error,omitempty"`
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
// into a single PackageSpec list in the result.  Each PackageSet defines the
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

type ErrorType string

const (
	DepsolveErrorType ErrorType = "depsolve"
	OtherErrorType    ErrorType = "other"
)

// SbomDoc represents a single SBOM document result.
type SbomDoc struct {
	DocType  sbom.StandardType `json:"type"`
	Document json.RawMessage   `json:"document"`
}

type DepsolveJobResult struct {
	PackageSpecs map[string][]rpmmd.PackageSpec `json:"package_specs"`
	SbomDocs     map[string]SbomDoc             `json:"sbom_docs,omitempty"`
	RepoConfigs  map[string][]rpmmd.RepoConfig  `json:"repo_configs"`
	Error        string                         `json:"error"`
	ErrorType    ErrorType                      `json:"error_type"`
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

type BootcManifestJob struct {
	ImageType string `json:"image_type"`
	Arch      string `json:"arch"`
	ImageRef  string `json:"image_ref"`
	TLSVerify bool   `json:"tls_verify,omitempty"`
}

type BootcManifestJobResult struct {
	Manifest manifest.OSBuildManifest `json:"data,omitempty"`
	Error    string                   `json:"error"`
	JobResult
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

//
// JSON-serializable types for the client
//

type updateJobRequest struct {
	Result interface{} `json:"result"`
}

func (j *OSBuildJob) UnmarshalJSON(data []byte) error {
	// handles unmarshalling old jobs in the queue that don't contain newer fields
	// adds default/fallback values to missing data
	type aliasType OSBuildJob
	type compatType struct {
		aliasType
		// Deprecated: Exports should not be used. The export is set in the `Target.OsbuildExport`
		Exports []string `json:"export_stages,omitempty"`
	}
	var compat compatType
	if err := json.Unmarshal(data, &compat); err != nil {
		return err
	}
	if compat.PipelineNames == nil {
		compat.PipelineNames = &PipelineNames{
			Build:   distro.BuildPipelinesFallback(),
			Payload: distro.PayloadPipelinesFallback(),
		}
	}

	// Exports used to be specified in the job, but there could be always only a single export specified.
	if len(compat.Exports) != 0 {
		if len(compat.Exports) > 1 {
			return fmt.Errorf("osbuild job has more than one exports specified")
		}
		export := compat.Exports[0]
		// add the single export to each target
		for idx := range compat.Targets {
			target := compat.Targets[idx]
			if target.OsbuildArtifact.ExportName == "" {
				target.OsbuildArtifact.ExportName = export
			} else if target.OsbuildArtifact.ExportName != export {
				return fmt.Errorf("osbuild job has different global exports and export in the target specified at the same time")
			}
			compat.Targets[idx] = target
		}
	}

	*j = OSBuildJob(compat.aliasType)
	return nil
}

func (j OSBuildJob) MarshalJSON() ([]byte, error) {
	type aliasType OSBuildJob
	type compatType struct {
		aliasType
		// Depredated: Exports should not be used. The export is set in the `Target.OsbuildExport`
		Exports []string `json:"export_stages,omitempty"`
	}
	compat := compatType{
		aliasType: aliasType(j),
	}
	compat.Exports = j.OsbuildExports()

	data, err := json.Marshal(&compat)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (j *OSBuildJobResult) UnmarshalJSON(data []byte) error {
	// handles unmarshalling old jobs in the queue that don't contain newer fields
	// adds default/fallback values to missing data
	type aliastype OSBuildJobResult
	var alias aliastype
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if alias.PipelineNames == nil {
		alias.PipelineNames = &PipelineNames{
			Build:   distro.BuildPipelinesFallback(),
			Payload: distro.PayloadPipelinesFallback(),
		}
	}
	*j = OSBuildJobResult(alias)
	return nil
}
