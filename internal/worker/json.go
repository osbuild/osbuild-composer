package worker

import (
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
)

//
// JSON-serializable types for the jobqueue
//

type OSBuildJob struct {
	Manifest        distro.Manifest  `json:"manifest"`
	Targets         []*target.Target `json:"targets,omitempty"`
	ImageName       string           `json:"image_name,omitempty"`
	StreamOptimized bool             `json:"stream_optimized,omitempty"`
	Exports         []string         `json:"export_stages,omitempty"`
}

type OSBuildJobResult struct {
	Success       bool                   `json:"success"`
	OSBuildOutput *osbuild.Result        `json:"osbuild_output,omitempty"`
	TargetResults []*target.TargetResult `json:"target_results,omitempty"`
	TargetErrors  []string               `json:"target_errors,omitempty"`
	UploadStatus  string                 `json:"upload_status"`
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
}

type OSBuildKojiJob struct {
	Manifest      distro.Manifest `json:"manifest"`
	ImageName     string          `json:"image_name"`
	Exports       []string        `json:"exports"`
	KojiServer    string          `json:"koji_server"`
	KojiDirectory string          `json:"koji_directory"`
	KojiFilename  string          `json:"koji_filename"`
}

type OSBuildKojiJobResult struct {
	HostOS        string          `json:"host_os"`
	Arch          string          `json:"arch"`
	OSBuildOutput *osbuild.Result `json:"osbuild_output"`
	ImageHash     string          `json:"image_hash"`
	ImageSize     uint64          `json:"image_size"`
	KojiError     string          `json:"koji_error"`
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
}

type DepsolveJob struct {
	PackageSets      map[string]rpmmd.PackageSet `json:"package_sets"`
	Repos            []rpmmd.RepoConfig          `json:"repos"`
	ModulePlatformID string                      `json:"module_platform_id"`
	Arch             string                      `json:"arch"`
	Releasever       string                      `json:"releasever"`
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
}

type ManifestJobByID struct{}

type ManifestJobByIDResult struct {
	Manifest distro.Manifest `json:"data,omitempty"`
	Error    string          `json:"error"`
}

//
// JSON-serializable types for the client
//

type updateJobRequest struct {
	Result interface{} `json:"result"`
}
