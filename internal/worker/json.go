package worker

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

//
// JSON-serializable types for the jobqueue
//

type OSBuildJob struct {
	Manifest distro.Manifest `json:"manifest,omitempty"`
	// Index of the ManifestJobByIDResult instance in the job's dynamic arguments slice
	ManifestDynArgsIdx *int             `json:"manifest_dyn_args_idx,omitempty"`
	Targets            []*target.Target `json:"targets,omitempty"`
	PipelineNames      *PipelineNames   `json:"pipeline_names,omitempty"`
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
			targetErrors = append(targetErrors, clienterrors.WorkerClientError(targetError.ID, targetError.Reason, targetResult.Name))
		}
	}

	return targetErrors
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

type OSBuildKojiJob struct {
	Manifest      distro.Manifest `json:"manifest,omitempty"`
	ImageName     string          `json:"image_name"`
	Exports       []string        `json:"exports"`
	PipelineNames *PipelineNames  `json:"pipeline_names,omitempty"`
	KojiServer    string          `json:"koji_server"`
	KojiDirectory string          `json:"koji_directory"`
	KojiFilename  string          `json:"koji_filename"`
}

type OSBuildKojiJobResult struct {
	HostOS        string          `json:"host_os"`
	Arch          string          `json:"arch"`
	OSBuildOutput *osbuild.Result `json:"osbuild_output"`
	PipelineNames *PipelineNames  `json:"pipeline_names,omitempty"`
	ImageHash     string          `json:"image_hash"`
	ImageSize     uint64          `json:"image_size"`
	KojiError     string          `json:"koji_error"` // not set by any code other than unit tests
	JobResult
}

type KojiFinalizeJob struct {
	Server        string   `json:"server"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Release       string   `json:"release"`
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
// // 2. A pipeline ordering when the lists are concatenated: build -> os
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
}

// Custom marshaller for keeping compatibility with older workers.  The
// serialised format encompasses both the old and new DepsolveJob formats.  It
// is meant to be temporarily used to transition from the old to the new
// format.
func (ds DepsolveJob) MarshalJSON() ([]byte, error) {
	// NOTE: Common, top level repositories aren't used because in the new
	// format they don't exist; putting all required repositories on all
	// package sets as PackageSetsRepos should produce the same behaviour so
	// there's no need to try and figure out the "common" ones.
	// This also makes it possible to use old workers for new image types that
	// are incompatible with having common repos for all package sets (RHEL 7.9).
	compatJob := struct {
		// new format
		GroupedPackageSets map[string][]rpmmd.PackageSet `json:"grouped_package_sets"`
		ModulePlatformID   string                        `json:"module_platform_id"`
		Arch               string                        `json:"arch"`
		Releasever         string                        `json:"releasever"`

		// old format elements
		PackageSetsChains map[string][]string           `json:"package_sets_chains"`
		PackageSets       map[string]rpmmd.PackageSet   `json:"package_sets"`
		PackageSetsRepos  map[string][]rpmmd.RepoConfig `json:"package_sets_repositories,omitempty"`
	}{
		// new format substruct
		GroupedPackageSets: ds.PackageSets,
		ModulePlatformID:   ds.ModulePlatformID,
		Arch:               ds.Arch,
		Releasever:         ds.Releasever,
	}

	// build equivalent old format substruct
	pkgSetRepos := make(map[string][]rpmmd.RepoConfig)
	pkgSets := make(map[string]rpmmd.PackageSet)
	chains := make(map[string][]string)
	for chainName, pkgSetChain := range ds.PackageSets {
		if len(pkgSetChain) == 1 {
			// single element "chain" (i.e., not a chain)
			pkgSets[chainName] = rpmmd.PackageSet{
				Include: pkgSetChain[0].Include,
				Exclude: pkgSetChain[0].Exclude,
			}
			pkgSetRepos[chainName] = pkgSetChain[0].Repositories
			continue
		}
		chain := make([]string, len(pkgSetChain))
		for idx, set := range pkgSetChain {
			// the names of the individual sets in the chain don't matter, as long
			// as they match the keys for the repo configs
			setName := fmt.Sprintf("%s-%d", chainName, idx)
			// the package set (without repos)
			pkgSets[setName] = rpmmd.PackageSet{
				Include: set.Include,
				Exclude: set.Exclude,
			}
			// set repositories
			pkgSetRepos[setName] = set.Repositories
			// add name to the chain
			chain[idx] = setName
		}
		chains[chainName] = chain
	}

	compatJob.PackageSets = pkgSets
	compatJob.PackageSetsChains = chains
	compatJob.PackageSetsRepos = pkgSetRepos

	return json.Marshal(compatJob)
}

type ErrorType string

const (
	DepsolveErrorType ErrorType = "depsolve"
	OtherErrorType    ErrorType = "other"
)

type DepsolveJobResult struct {
	PackageSpecs map[string][]rpmmd.PackageSpec `json:"package_specs"`
	Error        string                         `json:"error"`
	ErrorType    ErrorType                      `json:"error_type"`
	JobResult
}

type ManifestJobByID struct{}

type ManifestJobByIDResult struct {
	Manifest distro.Manifest `json:"data,omitempty"`
	Error    string          `json:"error"`
	JobResult
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

func (j *OSBuildKojiJob) UnmarshalJSON(data []byte) error {
	// handles unmarshalling old jobs in the queue that don't contain newer fields
	// adds default/fallback values to missing data
	type aliastype OSBuildKojiJob
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
	*j = OSBuildKojiJob(alias)
	return nil
}

func (j *OSBuildKojiJobResult) UnmarshalJSON(data []byte) error {
	// handles unmarshalling old jobs in the queue that don't contain newer fields
	// adds default/fallback values to missing data
	type aliastype OSBuildKojiJobResult
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
	*j = OSBuildKojiJobResult(alias)
	return nil
}
