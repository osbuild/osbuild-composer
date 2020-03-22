// Package store contains primitives for representing and changing the
// osbuild-composer state.
package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/osbuild/osbuild-composer/internal/compose"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
)

// A Store contains all the persistent state of osbuild-composer, and is serialized
// on every change, and deserialized on start.
type Store struct {
	Blueprints        map[string]blueprint.Blueprint         `json:"blueprints"`
	Workspace         map[string]blueprint.Blueprint         `json:"workspace"`
	Composes          map[uuid.UUID]compose.Compose          `json:"composes"`
	Sources           map[string]SourceConfig                `json:"sources"`
	BlueprintsChanges map[string]map[string]blueprint.Change `json:"changes"`
	BlueprintsCommits map[string][]string                    `json:"commits"`

	mu             sync.RWMutex // protects all fields
	pendingJobs    chan Job
	stateChannel   chan []byte
	distroRegistry distro.Registry
	stateDir       *string
}

// A Job contains the information about a compose a worker needs to process it.
type Job struct {
	ComposeID    uuid.UUID
	ImageBuildID int
	Manifest     *osbuild.Manifest
	Targets      []*target.Target
	ImageType    string
}

type SourceConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	CheckGPG bool   `json:"check_gpg"`
	CheckSSL bool   `json:"check_ssl"`
	System   bool   `json:"system"`
}

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return e.message
}

type NotPendingError struct {
	message string
}

func (e *NotPendingError) Error() string {
	return e.message
}

type NotRunningError struct {
	message string
}

func (e *NotRunningError) Error() string {
	return e.message
}

type InvalidRequestError struct {
	message string
}

func (e *InvalidRequestError) Error() string {
	return e.message
}

type NoLocalTargetError struct {
	message string
}

func (e *NoLocalTargetError) Error() string {
	return e.message
}

func New(stateDir *string, distroRegistryArg distro.Registry) *Store {
	var s Store

	if stateDir != nil {
		err := os.Mkdir(*stateDir+"/"+"outputs", 0700)
		if err != nil && !os.IsExist(err) {
			log.Fatalf("cannot create output directory")
		}

		stateFile := *stateDir + "/state.json"
		state, err := ioutil.ReadFile(stateFile)

		if err == nil {
			err := json.Unmarshal(state, &s)
			if err != nil {
				log.Fatalf("invalid initial state: %v", err)
			}
		} else if !os.IsNotExist(err) {
			log.Fatalf("cannot read state: %v", err)
		}

		s.stateChannel = make(chan []byte, 128)

		go func() {
			for {
				err := writeFileAtomically(stateFile, <-s.stateChannel, 0600)
				if err != nil {
					log.Fatalf("cannot write state: %v", err)
				}
			}
		}()
	}

	s.pendingJobs = make(chan Job, 200)
	s.distroRegistry = distroRegistryArg
	s.stateDir = stateDir

	if s.Blueprints == nil {
		s.Blueprints = make(map[string]blueprint.Blueprint)
	}
	if s.Workspace == nil {
		s.Workspace = make(map[string]blueprint.Blueprint)
	}
	if s.Composes == nil {
		s.Composes = make(map[uuid.UUID]compose.Compose)
	} else {
		for composeID, compose := range s.Composes {
			if len(compose.ImageBuilds) == 0 {
				panic("the was a compose with zero image builds, that is forbidden")
			}
			for _, imgBuild := range compose.ImageBuilds {
				switch imgBuild.QueueStatus {
				case common.IBRunning:
					// We do not support resuming an in-flight build
					imgBuild.QueueStatus = common.IBFailed
					// s.Composes[composeID] = compose
				case common.IBWaiting:
					// Push waiting composes back into the pending jobs queue
					imageTypeCompat, exists := imgBuild.ImageType.ToCompatString()
					if !exists {
						panic("fatal error, image type tag should exist but does not")
					}
					s.pendingJobs <- Job{
						ComposeID:    composeID,
						ImageBuildID: imgBuild.Id,
						Manifest:     imgBuild.Manifest,
						Targets:      imgBuild.Targets,
						ImageType:    imageTypeCompat,
					}
				}
			}
		}
	}
	if s.Sources == nil {
		s.Sources = make(map[string]SourceConfig)
	}
	if s.BlueprintsChanges == nil {
		s.BlueprintsChanges = make(map[string]map[string]blueprint.Change)
	}
	if s.BlueprintsCommits == nil {
		s.BlueprintsCommits = make(map[string][]string)
	}

	// Populate BlueprintsCommits for existing blueprints without commit history
	// BlueprintsCommits tracks the order of the commits in BlueprintsChanges,
	// but may not be in-sync with BlueprintsChanges because it was added later.
	// This will sort the existing commits by timestamp and version to update
	// the store. BUT since the timestamp resolution is only 1s it is possible
	// that the order may be slightly wrong.
	for name := range s.BlueprintsChanges {
		if len(s.BlueprintsChanges[name]) != len(s.BlueprintsCommits[name]) {
			changes := make([]blueprint.Change, 0, len(s.BlueprintsChanges[name]))

			for commit := range s.BlueprintsChanges[name] {
				changes = append(changes, s.BlueprintsChanges[name][commit])
			}

			// Sort the changes by Timestamp then version, ascending
			sort.Slice(changes, func(i, j int) bool {
				if changes[i].Timestamp == changes[j].Timestamp {
					vI, err := semver.NewVersion(changes[i].Blueprint.Version)
					if err != nil {
						vI = semver.New("0.0.0")
					}
					vJ, err := semver.NewVersion(changes[j].Blueprint.Version)
					if err != nil {
						vJ = semver.New("0.0.0")
					}

					return vI.LessThan(*vJ)
				}
				return changes[i].Timestamp < changes[j].Timestamp
			})

			commits := make([]string, 0, len(changes))
			for _, c := range changes {
				commits = append(commits, c.Commit)
			}

			s.BlueprintsCommits[name] = commits
		}
	}

	return &s
}

func writeFileAtomically(filename string, data []byte, mode os.FileMode) error {
	dir, name := filepath.Dir(filename), filepath.Base(filename)

	tmpfile, err := ioutil.TempFile(dir, name+"-*.tmp")
	if err != nil {
		return err
	}

	_, err = tmpfile.Write(data)
	if err != nil {
		// FIXME: handle or comment this possible error
		_ = os.Remove(tmpfile.Name())
		return err
	}

	err = tmpfile.Chmod(mode)
	if err != nil {
		return err
	}

	err = tmpfile.Close()
	if err != nil {
		// FIXME: handle or comment this possible error
		_ = os.Remove(tmpfile.Name())
		return err
	}

	err = os.Rename(tmpfile.Name(), filename)
	if err != nil {
		// FIXME: handle or comment this possible error
		_ = os.Remove(tmpfile.Name())
		return err
	}

	return nil
}

func randomSHA1String() (string, error) {
	hash := sha1.New()
	data := make([]byte, 20)
	n, err := rand.Read(data)
	if err != nil {
		return "", err
	} else if n != 20 {
		return "", errors.New("randomSHA1String: short read from rand")
	}
	_, err = hash.Write(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Store) change(f func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := f()

	if s.stateChannel != nil {
		serialized, err := json.Marshal(s)
		if err != nil {
			// we ought to know all types that go into the store
			panic(err)
		}

		s.stateChannel <- serialized
	}
	return result
}

func (s *Store) ListBlueprints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Blueprints))
	for name := range s.Blueprints {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (s *Store) GetBlueprint(name string) (*blueprint.Blueprint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bp, inWorkspace := s.Workspace[name]
	if !inWorkspace {
		var ok bool
		bp, ok = s.Blueprints[name]
		if !ok {
			return nil, false
		}
	}

	return &bp, inWorkspace
}

func (s *Store) GetBlueprintCommitted(name string) *blueprint.Blueprint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bp, ok := s.Blueprints[name]
	if !ok {
		return nil
	}

	return &bp
}

// GetBlueprintChange returns a specific change to a blueprint
// If the blueprint or change do not exist then an error is returned
func (s *Store) GetBlueprintChange(name string, commit string) (*blueprint.Change, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.BlueprintsChanges[name]; !ok {
		return nil, errors.New("Unknown blueprint")
	}
	change, ok := s.BlueprintsChanges[name][commit]
	if !ok {
		return nil, errors.New("Unknown commit")
	}
	return &change, nil
}

// GetBlueprintChanges returns the list of changes, oldest first
func (s *Store) GetBlueprintChanges(name string) []blueprint.Change {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var changes []blueprint.Change

	for _, commit := range s.BlueprintsCommits[name] {
		changes = append(changes, s.BlueprintsChanges[name][commit])
	}

	return changes
}

func (s *Store) PushBlueprint(bp blueprint.Blueprint, commitMsg string) error {
	return s.change(func() error {
		commit, err := randomSHA1String()
		if err != nil {
			return err
		}

		// Make sure the blueprint has default values and that the version is valid
		err = bp.Initialize()
		if err != nil {
			return err
		}

		timestamp := time.Now().Format("2006-01-02T15:04:05Z")
		change := blueprint.Change{
			Commit:    commit,
			Message:   commitMsg,
			Timestamp: timestamp,
			Blueprint: bp,
		}

		delete(s.Workspace, bp.Name)
		if s.BlueprintsChanges[bp.Name] == nil {
			s.BlueprintsChanges[bp.Name] = make(map[string]blueprint.Change)
		}
		s.BlueprintsChanges[bp.Name][commit] = change
		// Keep track of the order of the commits
		s.BlueprintsCommits[bp.Name] = append(s.BlueprintsCommits[bp.Name], commit)

		if old, ok := s.Blueprints[bp.Name]; ok {
			if bp.Version == "" || bp.Version == old.Version {
				bp.BumpVersion(old.Version)
			}
		}
		s.Blueprints[bp.Name] = bp
		return nil
	})
}

func (s *Store) PushBlueprintToWorkspace(bp blueprint.Blueprint) error {
	return s.change(func() error {
		// Make sure the blueprint has default values and that the version is valid
		err := bp.Initialize()
		if err != nil {
			return err
		}

		s.Workspace[bp.Name] = bp
		return nil
	})
}

// DeleteBlueprint will remove the named blueprint from the store
// if the blueprint does not exist it will return an error
// The workspace copy is deleted unconditionally, it will not return an error if it does not exist.
func (s *Store) DeleteBlueprint(name string) error {
	return s.change(func() error {
		delete(s.Workspace, name)
		if _, ok := s.Blueprints[name]; !ok {
			return fmt.Errorf("Unknown blueprint: %s", name)
		}
		delete(s.Blueprints, name)
		return nil
	})
}

// DeleteBlueprintFromWorkspace deletes the workspace copy of a blueprint
// if the blueprint doesn't exist in the workspace it returns an error
func (s *Store) DeleteBlueprintFromWorkspace(name string) error {
	return s.change(func() error {
		if _, ok := s.Workspace[name]; !ok {
			return fmt.Errorf("Unknown blueprint: %s", name)
		}
		delete(s.Workspace, name)
		return nil
	})
}

// TagBlueprint will tag the most recent commit
// It will return an error if the blueprint doesn't exist
func (s *Store) TagBlueprint(name string) error {
	return s.change(func() error {
		_, ok := s.Blueprints[name]
		if !ok {
			return errors.New("Unknown blueprint")
		}

		if len(s.BlueprintsCommits[name]) == 0 {
			return errors.New("No commits for blueprint")
		}

		latest := s.BlueprintsCommits[name][len(s.BlueprintsCommits[name])-1]
		// If the most recent commit already has a revision, don't bump it
		if s.BlueprintsChanges[name][latest].Revision != nil {
			return nil
		}

		// Get the latest revision for this blueprint
		var revision int
		var change blueprint.Change
		for i := len(s.BlueprintsCommits[name]) - 1; i >= 0; i-- {
			commit := s.BlueprintsCommits[name][i]
			change = s.BlueprintsChanges[name][commit]
			if change.Revision != nil && *change.Revision > revision {
				revision = *change.Revision
				break
			}
		}

		// Bump the revision (if there was none it will start at 1)
		revision++
		change.Revision = &revision
		s.BlueprintsChanges[name][latest] = change
		return nil
	})
}

func (s *Store) GetCompose(id uuid.UUID) (compose.Compose, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compose, exists := s.Composes[id]
	return compose, exists
}

// GetAllComposes creates a deep copy of all composes present in this store
// and returns them as a dictionary with compose UUIDs as keys
func (s *Store) GetAllComposes() map[uuid.UUID]compose.Compose {
	s.mu.RLock()
	defer s.mu.RUnlock()

	composes := make(map[uuid.UUID]compose.Compose)

	for id, singleCompose := range s.Composes {
		newCompose := singleCompose.DeepCopy()
		composes[id] = newCompose
	}

	return composes
}

func (s *Store) GetImageBuildResult(composeId uuid.UUID, imageBuildId int) (io.ReadCloser, error) {
	if s.stateDir == nil {
		return ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))), nil
	}

	return os.Open(s.getImageBuildDirectory(composeId, imageBuildId) + "/result.json")
}

func (s *Store) GetImageBuildImage(composeId uuid.UUID, imageBuildId int) (io.ReadCloser, int64, error) {
	c, ok := s.Composes[composeId]

	if !ok {
		return nil, 0, &NotFoundError{"compose does not exist"}
	}

	localTargetOptions := c.ImageBuilds[imageBuildId].GetLocalTargetOptions()
	if localTargetOptions == nil {
		return nil, 0, &NoLocalTargetError{"compose does not have local target"}
	}

	path := fmt.Sprintf("%s/%s", s.getImageBuildDirectory(composeId, imageBuildId), localTargetOptions.Filename)

	f, err := os.Open(path)

	if err != nil {
		return nil, 0, err
	}

	fileInfo, err := f.Stat()

	if err != nil {
		return nil, 0, err
	}

	return f, fileInfo.Size(), err

}

func (s *Store) getComposeDirectory(composeID uuid.UUID) string {
	return fmt.Sprintf("%s/outputs/%s", *s.stateDir, composeID.String())
}

func (s *Store) getImageBuildDirectory(composeID uuid.UUID, imageBuildID int) string {
	return fmt.Sprintf("%s/%d", s.getComposeDirectory(composeID), imageBuildID)
}

func (s *Store) PushCompose(distro distro.Distro, composeID uuid.UUID, bp *blueprint.Blueprint, repos []rpmmd.RepoConfig, packages, buildPackages []rpmmd.PackageSpec, arch, composeType string, size uint64, uploadTarget *target.Target) error {
	targets := []*target.Target{}

	// Compatibility layer for image types in Weldr API v0
	imageType, exists := common.ImageTypeFromCompatString(composeType)
	if !exists {
		panic("fatal error, compose type does not exist")
	}

	if s.stateDir != nil {
		outputDir := s.getImageBuildDirectory(composeID, 0)

		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("cannot create output directory for job %v: %#v", composeID, err)
		}

		filename, _, err := distro.FilenameFromType(composeType)
		if err != nil {
			return fmt.Errorf("cannot query filename from image type %s: %#v", composeType, err)
		}

		targets = append(targets, target.NewLocalTarget(
			&target.LocalTargetOptions{
				Filename: filename,
			},
		))
	}

	size = distro.GetSizeForOutputType(composeType, size)

	if uploadTarget != nil {
		targets = append(targets, uploadTarget)
	}

	allRepos := append([]rpmmd.RepoConfig{}, repos...)
	for _, source := range s.Sources {
		allRepos = append(allRepos, source.RepoConfig())
	}

	manifestStruct, err := distro.Manifest(bp.Customizations, allRepos, packages, buildPackages, arch, composeType, size)
	if err != nil {
		return err
	}
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.Composes[composeID] = compose.Compose{
			Blueprint: bp,
			ImageBuilds: []compose.ImageBuild{
				{
					QueueStatus: common.IBWaiting,
					Manifest:    manifestStruct,
					ImageType:   imageType,
					Targets:     targets,
					JobCreated:  time.Now(),
					Size:        size,
				},
			},
		}
		return nil
	})
	s.pendingJobs <- Job{
		ComposeID:    composeID,
		ImageBuildID: 0,
		Manifest:     manifestStruct,
		Targets:      targets,
		ImageType:    composeType,
	}

	return nil
}

// DeleteCompose deletes the compose from the state file and also removes all files on disk that are
// associated with this compose
func (s *Store) DeleteCompose(id uuid.UUID) error {
	return s.change(func() error {
		compose, exists := s.Composes[id]

		if !exists {
			return &NotFoundError{}
		}

		// If any of the image builds have build artifacts, remove them
		invalidRequest := true
		for _, imageBuild := range compose.ImageBuilds {
			if imageBuild.QueueStatus == common.IBFinished || imageBuild.QueueStatus == common.IBFailed {
				invalidRequest = false
			}
		}
		if invalidRequest {
			return &InvalidRequestError{fmt.Sprintf("Compose %s is not in FINISHED or FAILED.", id)}
		}

		delete(s.Composes, id)

		var err error
		if s.stateDir != nil {
			// TODO: we need to rename the files as the compose will have multiple images
			for _, imageBuild := range compose.ImageBuilds {
				if imageBuild.QueueStatus == common.IBFinished || imageBuild.QueueStatus == common.IBFailed {
					err = os.RemoveAll(s.getComposeDirectory(id))
					if err != nil {
						return err
					}
				}
			}
		}

		return err
	})
}

// PopJob returns a job from the job queue and changes the status of the corresponding image build to running
func (s *Store) PopJob() Job {
	job := <-s.pendingJobs
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		// Get the compose from the map
		compose, exists := s.Composes[job.ComposeID]
		// Check that it exists
		if !exists {
			panic("Invalid job in queue.")
		}
		// Loop over the image builds and find the right one according to the image type
		// Change queue status to running for the image build as well as for the targets
		for n, imageBuild := range compose.ImageBuilds {
			imgTypeCompatStr, exists := imageBuild.ImageType.ToCompatString()
			if !exists {
				panic("fatal error, image type should exist but it does not")
			}
			if imgTypeCompatStr != job.ImageType {
				continue
			}
			if imageBuild.QueueStatus != common.IBWaiting {
				panic("Invalid job in queue.")
			}
			compose.ImageBuilds[n].QueueStatus = common.IBRunning
			for m := range compose.ImageBuilds[n].Targets {
				compose.ImageBuilds[n].Targets[m].Status = common.IBRunning
			}
		}
		// Replace the compose struct with the new one
		// TODO: I'm not sure this is needed, but I don't know what is the golang semantics in this case
		s.Composes[job.ComposeID] = compose
		return nil
	})
	return job
}

// UpdateImageBuildInCompose sets the status and optionally also the final image.
func (s *Store) UpdateImageBuildInCompose(composeID uuid.UUID, imageBuildID int, status common.ImageBuildState, result *common.ComposeResult) error {
	return s.change(func() error {
		// Check that the compose exists
		currentCompose, exists := s.Composes[composeID]
		if !exists {
			return &NotFoundError{"compose does not exist"}
		}
		// Check that the image build was waiting
		if currentCompose.ImageBuilds[imageBuildID].QueueStatus == common.IBWaiting {
			return &NotPendingError{"compose has not been popped"}
		}

		// write result into file
		if s.stateDir != nil && result != nil {
			f, err := os.Create(s.getImageBuildDirectory(composeID, imageBuildID) + "/result.json")

			if err != nil {
				return fmt.Errorf("cannot open result.json for job %v: %#v", composeID, err)
			}

			// FIXME: handle error
			_ = json.NewEncoder(f).Encode(result)
		}

		// Update the image build state including all target states
		err := currentCompose.UpdateState(imageBuildID, status)
		if err != nil {
			// TODO: log error
			return &InvalidRequestError{"invalid state transition: " + err.Error()}
		}

		// In case the image build is done, store the time and possibly also the image
		if status == common.IBFinished || status == common.IBFailed {
			currentCompose.ImageBuilds[imageBuildID].JobFinished = time.Now()
		}

		s.Composes[composeID] = currentCompose

		return nil
	})
}

func (s *Store) AddImageToImageUpload(composeID uuid.UUID, imageBuildID int, reader io.Reader) error {
	currentCompose, exists := s.Composes[composeID]
	if !exists {
		return &NotFoundError{"compose does not exist"}
	}

	localTargetOptions := currentCompose.ImageBuilds[imageBuildID].GetLocalTargetOptions()
	if localTargetOptions == nil {
		return &NoLocalTargetError{fmt.Sprintf("image upload requested for compse %s and image build %d but it has no local target", composeID.String(), imageBuildID)}
	}

	path := fmt.Sprintf("%s/%s", s.getImageBuildDirectory(composeID, imageBuildID), localTargetOptions.Filename)
	f, err := os.Create(path)

	if err != nil {
		return err
	}

	_, err = io.Copy(f, reader)

	if err != nil {
		return err
	}

	return nil
}

func (s *Store) PushSource(source SourceConfig) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.Sources[source.Name] = source
		return nil
	})
}

func (s *Store) DeleteSource(name string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		delete(s.Sources, name)
		return nil
	})
}

func (s *Store) ListSources() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.Sources))
	for name := range s.Sources {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (s *Store) GetSource(name string) *SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source, ok := s.Sources[name]
	if !ok {
		return nil
	}
	return &source
}

func (s *Store) GetAllSources() map[string]SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make(map[string]SourceConfig)

	for k, v := range s.Sources {
		sources[k] = v
	}

	return sources
}

func NewSourceConfig(repo rpmmd.RepoConfig, system bool) SourceConfig {
	sc := SourceConfig{
		Name:     repo.Id,
		CheckGPG: true,
		CheckSSL: !repo.IgnoreSSL,
		System:   system,
	}

	if repo.BaseURL != "" {
		sc.URL = repo.BaseURL
		sc.Type = "yum-baseurl"
	} else if repo.Metalink != "" {
		sc.URL = repo.Metalink
		sc.Type = "yum-metalink"
	} else if repo.MirrorList != "" {
		sc.URL = repo.MirrorList
		sc.Type = "yum-mirrorlist"
	}

	return sc
}

func (s *SourceConfig) RepoConfig() rpmmd.RepoConfig {
	var repo rpmmd.RepoConfig

	repo.Id = s.Name
	repo.IgnoreSSL = !s.CheckSSL

	if s.Type == "yum-baseurl" {
		repo.BaseURL = s.URL
	} else if s.Type == "yum-metalink" {
		repo.Metalink = s.URL
	} else if s.Type == "yum-mirrorlist" {
		repo.MirrorList = s.URL
	}

	return repo
}
