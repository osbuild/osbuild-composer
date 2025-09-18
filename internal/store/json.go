package store

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/weldrtypes"
)

var getHostDistroName func() (string, error) = distro.GetHostDistroName
var getHostArch func() string = arch.Current().String

type storeV0 struct {
	Blueprints blueprintsV0 `json:"blueprints"`
	Workspace  workspaceV0  `json:"workspace"`
	Composes   composesV0   `json:"composes"`
	Sources    sourcesV0    `json:"sources"`
	Changes    changesV0    `json:"changes"`
	Commits    commitsV0    `json:"commits"`
}

type blueprintsV0 map[string]blueprint.Blueprint
type workspaceV0 map[string]blueprint.Blueprint

// A Compose represent the task of building a set of images from a single blueprint.
// It contains all the information necessary to generate the inputs for the job, as
// well as the job's state.
type composeV0 struct {
	Blueprint   *blueprint.Blueprint              `json:"blueprint"`
	ImageBuilds []imageBuildV0                    `json:"image_builds"`
	Packages    []weldrtypes.DepsolvedPackageInfo `json:"packages"`
}

type composesV0 map[uuid.UUID]composeV0

// ImageBuild represents a single image build inside a compose
type imageBuildV0 struct {
	ID          int                      `json:"id"`
	ImageType   string                   `json:"image_type"`
	Manifest    manifest.OSBuildManifest `json:"manifest"`
	Targets     []*target.Target         `json:"targets"`
	JobCreated  time.Time                `json:"job_created"`
	JobStarted  time.Time                `json:"job_started"`
	JobFinished time.Time                `json:"job_finished"`
	Size        uint64                   `json:"size"`
	JobID       uuid.UUID                `json:"jobid,omitempty"`

	// Kept for backwards compatibility. Image builds which were done
	// before the move to the job queue use this to store whether they
	// finished successfully.
	QueueStatus common.ImageBuildState `json:"queue_status,omitempty"`
}

type sourceV0 struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	URL            string   `json:"url"`
	CheckGPG       bool     `json:"check_gpg"`
	CheckSSL       bool     `json:"check_ssl"`
	System         bool     `json:"system"`
	Distros        []string `json:"distros"`
	RHSM           bool     `json:"rhsm"`
	CheckRepoGPG   bool     `json:"check_repogpg"`
	GPGKeys        []string `json:"gpgkeys"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
}

type sourcesV0 map[string]sourceV0

type changeV0 struct {
	Commit    string              `json:"commit"`
	Message   string              `json:"message"`
	Revision  *int                `json:"revision"`
	Timestamp string              `json:"timestamp"`
	Blueprint blueprint.Blueprint `json:"blueprint"`
}

type changesV0 map[string]map[string]changeV0

type commitsV0 map[string][]string

func newBlueprintsFromV0(blueprintsStruct blueprintsV0) map[string]blueprint.Blueprint {
	blueprints := make(map[string]blueprint.Blueprint)
	for name, blueprint := range blueprintsStruct {
		blueprints[name] = blueprint.DeepCopy()
	}
	return blueprints
}

func newWorkspaceFromV0(workspaceStruct workspaceV0) map[string]blueprint.Blueprint {
	workspace := make(map[string]blueprint.Blueprint)
	for name, blueprint := range workspaceStruct {
		workspace[name] = blueprint.DeepCopy()
	}
	return workspace
}

func newComposesFromV0(composesStruct composesV0, df *distrofactory.Factory, log *log.Logger) map[uuid.UUID]weldrtypes.Compose {
	composes := make(map[uuid.UUID]weldrtypes.Compose)

	for composeID, composeStruct := range composesStruct {
		c, err := newComposeFromV0(composeStruct, df)
		if err != nil {
			if log != nil {
				log.Printf("ignoring compose: %v", err)
			}
			continue
		}
		composes[composeID] = c
	}

	return composes
}

func newImageBuildFromV0(imageBuildStruct imageBuildV0, arch distro.Arch) (weldrtypes.ImageBuild, error) {
	imgType := imageTypeFromCompatString(imageBuildStruct.ImageType, arch)
	if imgType == nil {
		// Invalid type strings in serialization format, this may happen
		// on upgrades.
		return weldrtypes.ImageBuild{}, errors.New("invalid Image Type string")
	}
	// Backwards compatibility: fail all builds that are queued or
	// running. Jobs status is now handled outside of the store
	// (and the compose). The fields are kept so that previously
	// succeeded builds still show up correctly.
	queueStatus := imageBuildStruct.QueueStatus
	switch queueStatus {
	case common.IBRunning, common.IBWaiting:
		queueStatus = common.IBFailed
	}
	return weldrtypes.ImageBuild{
		ID:          imageBuildStruct.ID,
		ImageType:   imgType,
		Manifest:    imageBuildStruct.Manifest,
		Targets:     imageBuildStruct.Targets,
		JobCreated:  imageBuildStruct.JobCreated,
		JobStarted:  imageBuildStruct.JobStarted,
		JobFinished: imageBuildStruct.JobFinished,
		Size:        imageBuildStruct.Size,
		JobID:       imageBuildStruct.JobID,
		QueueStatus: queueStatus,
	}, nil
}

func newComposeFromV0(composeStruct composeV0, df *distrofactory.Factory) (weldrtypes.Compose, error) {
	if len(composeStruct.ImageBuilds) != 1 {
		return weldrtypes.Compose{}, errors.New("compose with unsupported number of image builds")
	}

	// Get the distro from the blueprint (empty means use host distro)
	bp := composeStruct.Blueprint.DeepCopy()
	distroName := bp.Distro
	if len(distroName) == 0 {
		var err error
		distroName, err = getHostDistroName()
		if err != nil {
			return weldrtypes.Compose{}, fmt.Errorf("Failed to get host distro name: %v", err)
		}
	}
	distro := df.GetDistro(distroName)
	if distro == nil {
		return weldrtypes.Compose{}, fmt.Errorf("Unknown distro - %s", distroName)
	}

	// Get the host distro's architecture. This contains the distro+arch specific image types
	arch, err := distro.GetArch(getHostArch())
	if err != nil {
		return weldrtypes.Compose{}, err
	}

	ib, err := newImageBuildFromV0(composeStruct.ImageBuilds[0], arch)
	if err != nil {
		return weldrtypes.Compose{}, err
	}

	pkgs := make([]weldrtypes.DepsolvedPackageInfo, len(composeStruct.Packages))
	copy(pkgs, composeStruct.Packages)

	return weldrtypes.Compose{
		Blueprint:  &bp,
		ImageBuild: ib,
		Packages:   pkgs,
	}, nil
}

func newSourceConfigsFromV0(sourcesStruct sourcesV0) map[string]SourceConfig {
	sources := make(map[string]SourceConfig)

	for name, source := range sourcesStruct {
		sources[name] = SourceConfig(source)
	}

	return sources
}

func newChangesFromV0(changesStruct changesV0) map[string]map[string]blueprint.Change {
	changes := make(map[string]map[string]blueprint.Change)

	for name, commitsStruct := range changesStruct {
		commits := make(map[string]blueprint.Change)
		for commitID, change := range commitsStruct {
			commits[commitID] = blueprint.Change{
				Commit:    change.Commit,
				Message:   change.Message,
				Revision:  change.Revision,
				Timestamp: change.Timestamp,
				Blueprint: change.Blueprint,
			}
		}
		changes[name] = commits
	}

	return changes
}

func newCommitsFromV0(commitsMapStruct commitsV0, changesMapStruct changesV0) map[string][]string {
	commitsMap := make(map[string][]string)
	for name, commitsStruct := range commitsMapStruct {
		commits := make([]string, len(commitsStruct))
		copy(commits, commitsStruct)
		commitsMap[name] = commits
	}

	// Populate BlueprintsCommits for existing blueprints without commit history
	// BlueprintsCommits tracks the order of the commits in BlueprintsChanges,
	// but may not be in-sync with BlueprintsChanges because it was added later.
	// This will sort the existing commits by timestamp and version to update
	// the store. BUT since the timestamp resolution is only 1s it is possible
	// that the order may be slightly wrong.
	for name, changes := range changesMapStruct {
		if _, exists := commitsMap[name]; !exists {
			changesSlice := make([]changeV0, 0, len(changes))

			// Copy the change objects from a map to a sortable slice
			for _, change := range changes {
				changesSlice = append(changesSlice, change)
			}

			// Sort the changes by Timestamp ascending
			sort.Slice(changesSlice, func(i, j int) bool {
				return changesSlice[i].Timestamp <= changesSlice[j].Timestamp
			})

			// Create a sorted list of commits based on the sorted list of change objects
			commits := make([]string, 0, len(changes))
			for _, c := range changesSlice {
				commits = append(commits, c.Commit)
			}

			// Assign the commits to the commit map, as an approximation of what we want
			commitsMap[name] = commits
		}
	}

	return commitsMap
}

func newStoreFromV0(storeStruct storeV0, df *distrofactory.Factory, log *log.Logger) *Store {
	return &Store{
		blueprints:        newBlueprintsFromV0(storeStruct.Blueprints),
		workspace:         newWorkspaceFromV0(storeStruct.Workspace),
		composes:          newComposesFromV0(storeStruct.Composes, df, log),
		sources:           newSourceConfigsFromV0(storeStruct.Sources),
		blueprintsChanges: newChangesFromV0(storeStruct.Changes),
		blueprintsCommits: newCommitsFromV0(storeStruct.Commits, storeStruct.Changes),
	}
}

func newBlueprintsV0(blueprints map[string]blueprint.Blueprint) blueprintsV0 {
	blueprintsStruct := make(blueprintsV0)
	for name, blueprint := range blueprints {
		blueprintsStruct[name] = blueprint.DeepCopy()
	}
	return blueprintsStruct
}

func newWorkspaceV0(workspace map[string]blueprint.Blueprint) workspaceV0 {
	workspaceStruct := make(workspaceV0)
	for name, blueprint := range workspace {
		workspaceStruct[name] = blueprint.DeepCopy()
	}
	return workspaceStruct
}

func newComposeV0(compose weldrtypes.Compose) composeV0 {
	bp := compose.Blueprint.DeepCopy()

	pkgs := make([]weldrtypes.DepsolvedPackageInfo, len(compose.Packages))
	copy(pkgs, compose.Packages)

	return composeV0{
		Blueprint: &bp,
		ImageBuilds: []imageBuildV0{
			{
				ID:          compose.ImageBuild.ID,
				ImageType:   imageTypeToCompatString(compose.ImageBuild.ImageType),
				Manifest:    compose.ImageBuild.Manifest,
				Targets:     compose.ImageBuild.Targets,
				JobCreated:  compose.ImageBuild.JobCreated,
				JobStarted:  compose.ImageBuild.JobStarted,
				JobFinished: compose.ImageBuild.JobFinished,
				Size:        compose.ImageBuild.Size,
				JobID:       compose.ImageBuild.JobID,
				QueueStatus: compose.ImageBuild.QueueStatus,
			},
		},
		Packages: pkgs,
	}
}

func newComposesV0(composes map[uuid.UUID]weldrtypes.Compose) composesV0 {
	composesStruct := make(composesV0)
	for composeID, compose := range composes {
		composesStruct[composeID] = newComposeV0(compose)
	}
	return composesStruct
}

func newSourcesV0(sources map[string]SourceConfig) sourcesV0 {
	sourcesStruct := make(sourcesV0)
	for name, source := range sources {
		sourcesStruct[name] = sourceV0(source)
	}
	return sourcesStruct
}

func newChangesV0(changes map[string]map[string]blueprint.Change) changesV0 {
	changesStruct := make(changesV0)
	for name, commits := range changes {
		commitsStruct := make(map[string]changeV0)
		for commitID, change := range commits {
			commitsStruct[commitID] = changeV0{
				Commit:    change.Commit,
				Message:   change.Message,
				Revision:  change.Revision,
				Timestamp: change.Timestamp,
				Blueprint: change.Blueprint,
			}
		}
		changesStruct[name] = commitsStruct
	}
	return changesStruct
}

func newCommitsV0(commits map[string][]string) commitsV0 {
	commitsStruct := make(commitsV0)
	for name, changes := range commits {
		commitsStruct[name] = changes
	}
	return commitsStruct
}

func (store *Store) toStoreV0() *storeV0 {
	return &storeV0{
		Blueprints: newBlueprintsV0(store.blueprints),
		Workspace:  newWorkspaceV0(store.workspace),
		Composes:   newComposesV0(store.composes),
		Sources:    newSourcesV0(store.sources),
		Changes:    newChangesV0(store.blueprintsChanges),
		Commits:    newCommitsV0(store.blueprintsCommits),
	}
}

var imageTypeCompatMapping = map[string]string{
	"vhd":              "Azure",
	"ami":              "AWS",
	"liveiso":          "LiveISO",
	"openstack":        "OpenStack",
	"vmdk":             "VMWare",
	"ext4-filesystem":  "Raw-filesystem",
	"partitioned-disk": "Partitioned-disk",
	"tar":              "Tar",
	"gce":              "GCP",
	"gce-rhui":         "GCE RHUI",
	"minimal-raw":      "minimal-raw",
}

func imageTypeToCompatString(imgType distro.ImageType) string {
	imgTypeString, exists := imageTypeCompatMapping[imgType.Name()]
	if !exists {
		// if no compat string exists, use the original name
		return imgType.Name()
	}
	return imgTypeString
}

func imageTypeFromCompatString(input string, arch distro.Arch) distro.ImageType {
	if arch == nil {
		return nil
	}

	// check if input string is a valid image type name: no compat mapping required
	if imgType, err := arch.GetImageType(input); err == nil {
		return imgType
	}

	for k, v := range imageTypeCompatMapping {
		if v == input {
			imgType, err := arch.GetImageType(k)
			if err != nil {
				return nil
			}
			return imgType
		}
	}

	return nil
}
