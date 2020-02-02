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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/osbuild/osbuild-composer/internal/compose"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"

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

	mu             sync.RWMutex // protects all fields
	pendingJobs    chan Job
	stateChannel   chan []byte
	distro         distro.Distro
	distroRegistry distro.Registry
	stateDir       *string
}

// A Job contains the information about a compose a worker needs to process it.
type Job struct {
	ComposeID    uuid.UUID
	ImageBuildID int
	Distro       string
	Pipeline     *osbuild.Pipeline
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

func New(stateDir *string, distroArg distro.Distro, distroRegistryArg distro.Registry) *Store {
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
	s.distro = distroArg
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
					distroStr, exists := imgBuild.Distro.ToString()
					if !exists {
						panic("fatal error, distro tag should exist but does not")
					}
					imageTypeCompat, exists := imgBuild.ImageType.ToCompatString()
					if !exists {
						panic("fatal error, image type tag should exist but does not")
					}
					s.pendingJobs <- Job{
						ComposeID:    composeID,
						ImageBuildID: imgBuild.Id,
						Distro:       distroStr,
						Pipeline:     imgBuild.Pipeline,
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
	hash.Write(data)
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

	// cockpit-composer cannot deal with missing "packages" or "modules"
	if bp.Packages == nil {
		bp.Packages = []blueprint.Package{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprint.Package{}
	}
	if bp.Groups == nil {
		bp.Groups = []blueprint.Group{}
	}
	if bp.Version == "" {
		bp.Version = "0.0.0"
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

	// cockpit-composer cannot deal with missing "packages" or "modules"
	if bp.Packages == nil {
		bp.Packages = []blueprint.Package{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprint.Package{}
	}
	if bp.Groups == nil {
		bp.Groups = []blueprint.Group{}
	}
	if bp.Version == "" {
		bp.Version = "0.0.0"
	}

	return &bp
}

func (s *Store) GetBlueprintChange(name string, commit string) *blueprint.Change {
	s.mu.RLock()
	defer s.mu.RUnlock()

	change, ok := s.BlueprintsChanges[name][commit]
	if !ok {
		return nil
	}
	return &change
}

func (s *Store) GetBlueprintChanges(name string) []blueprint.Change {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var changes []blueprint.Change

	for _, change := range s.BlueprintsChanges[name] {
		changes = append(changes, change)
	}

	return changes
}

func bumpVersion(str string) string {
	v := [3]uint64{}
	fields := strings.SplitN(str, ".", 3)
	for i := 0; i < len(fields); i++ {
		if n, err := strconv.ParseUint(fields[i], 10, 64); err == nil {
			v[i] = n
		} else {
			// don't touch strings with invalid versions
			return str
		}
	}

	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]+1)
}

func (s *Store) PushBlueprint(bp blueprint.Blueprint, commitMsg string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		commit, err := randomSHA1String()
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

		if old, ok := s.Blueprints[bp.Name]; ok {
			if bp.Version == "" || bp.Version == old.Version {
				bp.Version = bumpVersion(old.Version)
			}
		}
		s.Blueprints[bp.Name] = bp
		return nil
	})
}

func (s *Store) PushBlueprintToWorkspace(bp blueprint.Blueprint) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.Workspace[bp.Name] = bp
		return nil
	})
}

func (s *Store) DeleteBlueprint(name string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		delete(s.Workspace, name)
		delete(s.Blueprints, name)
		return nil
	})
}

func (s *Store) DeleteBlueprintFromWorkspace(name string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		delete(s.Workspace, name)
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

func (s *Store) GetComposeResult(id uuid.UUID) (io.ReadCloser, error) {
	if s.stateDir == nil {
		return ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))), nil
	}
	return os.Open(*s.stateDir + "/outputs/" + id.String() + "/result.json")
}

func (s *Store) getImageLocationForLocalTarget(composeID uuid.UUID) string {
	// This might result in conflicts because one compose can have multiple images, but (!) the Weldr API
	// does not support multiple images per build and the RCM API does not support Local target, so it does
	// not really matter any more.
	return *s.stateDir + "/outputs/" + composeID.String()
}

func (s *Store) PushCompose(composeID uuid.UUID, bp *blueprint.Blueprint, checksums map[string]string, arch, composeType string, size uint64, uploadTarget *target.Target) error {
	targets := []*target.Target{}

	// Compatibility layer for image types in Weldr API v0
	imageType, exists := common.ImageTypeFromCompatString(composeType)
	if !exists {
		panic("fatal error, compose type does not exist")
	}

	if s.stateDir != nil {
		outputDir := s.getImageLocationForLocalTarget(composeID)

		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("cannot create output directory for job %v: %#v", composeID, err)
		}

		targets = append(targets, target.NewLocalTarget(
			&target.LocalTargetOptions{
				Location: outputDir,
			},
		))
	}

	size = s.distro.GetSizeForOutputType(composeType, size)

	if uploadTarget != nil {
		targets = append(targets, uploadTarget)
	}

	repos := []rpmmd.RepoConfig{}
	for _, source := range s.Sources {
		repos = append(repos, source.RepoConfig())
	}

	pipelineStruct, err := s.distro.Pipeline(bp, repos, checksums, arch, composeType, size)
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
					Pipeline:    pipelineStruct,
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
		Distro:       s.distro.Name(),
		Pipeline:     pipelineStruct,
		Targets:      targets,
		ImageType:    composeType,
	}

	return nil
}

// PushComposeRequest is an alternative to PushCompose which does not assume a pre-defined distro, as such it is better
// suited for RCM API and possible future API that would respect the fact that we can build any distro and any arch
func (s *Store) PushComposeRequest(request common.ComposeRequest) error {
	// This should never happen and once distro.Pipeline is refactored this check will go away
	arch, exists := request.Arch.ToString()
	if !exists {
		panic("fatal error, arch should exist but it does not")
	}
	distroString, exists := request.Distro.ToString()
	if !exists {
		panic("fatal error, distro should exist but it does not")
	}

	distroStruct := s.distroRegistry.GetDistro(distroString)
	if distroStruct == nil {
		panic("fatal error, distro should exist but it is not in the registry")
	}

	// This will be a list of imageBuilds that will be submitted to the state channel
	imageBuilds := []compose.ImageBuild{}
	newJobs := []Job{}

	// TODO: remove this
	if len(request.RequestedImages) > 1 {
		panic("Multiple image requests are not yet properly implemented")
	}

	for n, imageRequest := range request.RequestedImages {
		// TODO: handle custom upload targets
		// TODO: this requires changes in the Compose Request struct
		// TODO: implment support for no checksums
		// This should never happen and once distro.Pipeline is refactored this check will go away
		imgTypeCompatStr, exists := imageRequest.ImgType.ToCompatString()
		if !exists {
			panic("fatal error, image type should exist but it does not")
		}
		pipelineStruct, err := distroStruct.Pipeline(&request.Blueprint, request.Repositories, nil, arch, imgTypeCompatStr, 0)
		if err != nil {
			return err
		}

		// This will make the job submission atomic: either all of them or none of them
		newJobs = append(newJobs, Job{
			ComposeID:    request.ComposeID,
			ImageBuildID: n,
			Distro:       distroString,
			Pipeline:     pipelineStruct,
			Targets:      []*target.Target{},
			ImageType:    imgTypeCompatStr,
		})

		imageBuilds = append(imageBuilds, compose.ImageBuild{
			Distro:      request.Distro,
			QueueStatus: common.IBWaiting,
			ImageType:   imageRequest.ImgType,
			Pipeline:    pipelineStruct,
			Targets:     []*target.Target{},
			JobCreated:  time.Now(),
		})
	}

	// submit all the jobs now
	for _, job := range newJobs {
		s.pendingJobs <- job
	}

	// ignore error because the previous implementation does the same
	_ = s.change(func() error {
		s.Composes[request.ComposeID] = compose.Compose{
			Blueprint:   &request.Blueprint,
			ImageBuilds: imageBuilds,
		}
		return nil
	})

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
					err = os.RemoveAll(s.getImageLocationForLocalTarget(id))
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
func (s *Store) UpdateImageBuildInCompose(composeID uuid.UUID, imageBuildID int, status common.ImageBuildState, image *compose.Image, result *common.ComposeResult) error {
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
			f, err := os.Create(s.getImageLocationForLocalTarget(composeID) + "/result.json")

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
			if status == common.IBFinished {
				currentCompose.ImageBuilds[imageBuildID].Image = image
			}
		}

		s.Composes[composeID] = currentCompose

		return nil
	})
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

	repo.Name = s.Name
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
