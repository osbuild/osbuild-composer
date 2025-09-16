// Package store contains primitives for representing and changing the
// osbuild-composer state.
package store

import (
	"crypto/rand"
	"strings"

	// The use of SHA1 is valid here
	/* #nosec G505 */
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/jsondb"
	"github.com/osbuild/osbuild-composer/internal/weldrtypes"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/google/uuid"
)

// StoreDBName is the name under which to save the store to the underlying jsondb
const StoreDBName = "state"

// A Store contains all the persistent state of osbuild-composer, and is serialized
// on every change, and deserialized on start.
//
// blueprints contain the most recent blueprint, using the name as the key
//
// blueprintsCommits contains the order of the commits to a blueprint name as a string of hashes
// with the most recent one last.
//
// blueprintsChanges contains the blueprint change, using the blueprint name string and
// the hash string from blueprintsCommits
type Store struct {
	blueprints        map[string]blueprint.Blueprint
	workspace         map[string]blueprint.Blueprint
	composes          map[uuid.UUID]weldrtypes.Compose
	sources           map[string]SourceConfig
	blueprintsChanges map[string]map[string]blueprint.Change
	blueprintsCommits map[string][]string

	mu       sync.RWMutex // protects all fields
	stateDir *string
	db       *jsondb.JSONDatabase
}

type SourceConfig struct {
	Name           string   `json:"name" toml:"name"`
	Type           string   `json:"type" toml:"type"`
	URL            string   `json:"url" toml:"url"`
	CheckGPG       bool     `json:"check_gpg" toml:"check_gpg"`
	CheckSSL       bool     `json:"check_ssl" toml:"check_ssl"`
	System         bool     `json:"system" toml:"system"`
	Distros        []string `json:"distros" toml:"distros"`
	RHSM           bool     `json:"rhsm" toml:"rhsm"`
	CheckRepoGPG   bool     `json:"check_repogpg" toml:"check_repogpg"`
	GPGKeys        []string `json:"gpgkeys"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
}

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return e.message
}

type NoLocalTargetError struct {
	message string
}

func (e *NoLocalTargetError) Error() string {
	return e.message
}

func New(stateDir *string, df *distrofactory.Factory, log *log.Logger) *Store {
	var storeStruct storeV0
	var db *jsondb.JSONDatabase

	if stateDir != nil {
		db = jsondb.New(*stateDir, 0600)
		_, err := db.Read(StoreDBName, &storeStruct)
		if err != nil && log != nil {
			log.Fatalf("cannot read state: %v", err)
		}
	}

	store := newStoreFromV0(storeStruct, df, log)

	store.stateDir = stateDir
	store.db = db

	return store
}

func randomSHA1String() (string, error) {
	// The use of SHA1 is accepted here
	/* #nosec G401 */
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

	if s.stateDir != nil {
		err := s.db.Write(StoreDBName, s.toStoreV0())
		if err != nil {
			panic(err)
		}
	}

	return result
}

func (s *Store) ListBlueprints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.blueprints))
	for name := range s.blueprints {
		if len(name) == 0 {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (s *Store) GetBlueprint(name string) (*blueprint.Blueprint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bp, inWorkspace := s.workspace[name]
	if !inWorkspace {
		var ok bool
		bp, ok = s.blueprints[name]
		if !ok {
			return nil, false
		}
	}

	return &bp, inWorkspace
}

func (s *Store) GetBlueprintCommitted(name string) *blueprint.Blueprint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bp, ok := s.blueprints[name]
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

	if _, ok := s.blueprintsChanges[name]; !ok {
		return nil, errors.New("Unknown blueprint")
	}
	change, ok := s.blueprintsChanges[name][commit]
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

	for _, commit := range s.blueprintsCommits[name] {
		changes = append(changes, s.blueprintsChanges[name][commit])
	}

	return changes
}

func (s *Store) PushBlueprint(bp blueprint.Blueprint, commitMsg string) error {
	return s.change(func() error {
		// Make sure the blueprint has default values and that the version is valid
		err := bp.Initialize()
		if err != nil {
			return err
		}

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

		delete(s.workspace, bp.Name)
		if s.blueprintsChanges[bp.Name] == nil {
			s.blueprintsChanges[bp.Name] = make(map[string]blueprint.Change)
		}
		s.blueprintsChanges[bp.Name][commit] = change
		// Keep track of the order of the commits
		s.blueprintsCommits[bp.Name] = append(s.blueprintsCommits[bp.Name], commit)

		if old, ok := s.blueprints[bp.Name]; ok {
			if bp.Version == "" || bp.Version == old.Version {
				bp.BumpVersion(old.Version)
			}
		}
		s.blueprints[bp.Name] = bp
		return nil
	})
}

func (s *Store) PushBlueprintToWorkspace(bp blueprint.Blueprint) error {
	return s.change(func() error {
		if len(bp.Name) == 0 {
			return fmt.Errorf("empty blueprint name not allowed")
		}

		// Make sure the blueprint has default values and that the version is valid
		err := bp.Initialize()
		if err != nil {
			return err
		}

		s.workspace[bp.Name] = bp
		return nil
	})
}

// DeleteBlueprint will remove the named blueprint from the store
// if the blueprint does not exist it will return an error
// The workspace copy is deleted unconditionally, it will not return an error if it does not exist.
func (s *Store) DeleteBlueprint(name string) error {
	return s.change(func() error {
		delete(s.workspace, name)
		if _, ok := s.blueprints[name]; !ok {
			return fmt.Errorf("Unknown blueprint: %s", name)
		}
		delete(s.blueprints, name)
		return nil
	})
}

// DeleteBlueprintFromWorkspace deletes the workspace copy of a blueprint
// if the blueprint doesn't exist in the workspace it returns an error
func (s *Store) DeleteBlueprintFromWorkspace(name string) error {
	return s.change(func() error {
		if _, ok := s.workspace[name]; !ok {
			return fmt.Errorf("Unknown blueprint: %s", name)
		}
		delete(s.workspace, name)
		return nil
	})
}

// TagBlueprint will tag the most recent commit
// It will return an error if the blueprint doesn't exist
func (s *Store) TagBlueprint(name string) error {
	return s.change(func() error {
		_, ok := s.blueprints[name]
		if !ok {
			return errors.New("Unknown blueprint")
		}

		_, ok = s.blueprintsCommits[name]
		if !ok || len(s.blueprintsCommits[name]) == 0 {
			return errors.New("No commits for blueprint")
		}

		_, ok = s.blueprintsChanges[name]
		if !ok {
			return errors.New("No changes for blueprint")
		}

		latest := s.blueprintsCommits[name][len(s.blueprintsCommits[name])-1]
		// If the most recent commit already has a revision, don't bump it
		if s.blueprintsChanges[name][latest].Revision != nil {
			return nil
		}

		// Get the latest revision for this blueprint (or 0 if there is none)
		// blueprintsCommits has the most recent commit at the end, so start there
		var revision int
		for i := len(s.blueprintsCommits[name]) - 1; i >= 0; i-- {
			commit := s.blueprintsCommits[name][i]
			change := s.blueprintsChanges[name][commit]
			if change.Revision != nil && *change.Revision > revision {
				revision = *change.Revision
				break
			}
		}

		// Bump the revision (if there was none it will start at 1) of the latest commit
		revision++
		change := s.blueprintsChanges[name][latest]
		change.Revision = &revision
		s.blueprintsChanges[name][latest] = change
		return nil
	})
}

func (s *Store) GetCompose(id uuid.UUID) (weldrtypes.Compose, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compose, exists := s.composes[id]
	return compose, exists
}

// GetAllComposes creates a deep copy of all composes present in this store
// and returns them as a dictionary with compose UUIDs as keys
func (s *Store) GetAllComposes() map[uuid.UUID]weldrtypes.Compose {
	s.mu.RLock()
	defer s.mu.RUnlock()

	composes := make(map[uuid.UUID]weldrtypes.Compose)

	for id, singleCompose := range s.composes {
		newCompose := singleCompose.DeepCopy()
		composes[id] = newCompose
	}

	return composes
}

func (s *Store) PushCompose(composeID uuid.UUID,
	manifest manifest.OSBuildManifest,
	imageType distro.ImageType,
	bp *blueprint.Blueprint,
	size uint64,
	targets []*target.Target,
	jobId uuid.UUID,
	packages []rpmmd.PackageSpec) error {

	if _, exists := s.GetCompose(composeID); exists {
		panic("a compose with this id already exists")
	}

	if targets == nil {
		targets = []*target.Target{}
	}

	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.composes[composeID] = weldrtypes.Compose{
			Blueprint: bp,
			ImageBuild: weldrtypes.ImageBuild{
				Manifest:   manifest,
				ImageType:  imageType,
				Targets:    targets,
				JobCreated: time.Now(),
				Size:       size,
				JobID:      jobId,
			},
			Packages: packages,
		}
		return nil
	})
	return nil
}

// PushTestCompose is used for testing
// Set testSuccess to create a fake successful compose, otherwise it will create a failed compose
// It does not actually run a compose job
func (s *Store) PushTestCompose(composeID uuid.UUID,
	manifest manifest.OSBuildManifest,
	imageType distro.ImageType,
	bp *blueprint.Blueprint,
	size uint64,
	targets []*target.Target,
	testSuccess bool,
	packages []rpmmd.PackageSpec) error {

	if targets == nil {
		targets = []*target.Target{}
	}

	var status common.ImageBuildState
	if testSuccess {
		status = common.IBFinished
	} else {
		status = common.IBFailed
	}

	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.composes[composeID] = weldrtypes.Compose{
			Blueprint: bp,
			ImageBuild: weldrtypes.ImageBuild{
				QueueStatus: status,
				Manifest:    manifest,
				ImageType:   imageType,
				Targets:     targets,
				JobCreated:  time.Now(),
				JobStarted:  time.Now(),
				Size:        size,
			},
			Packages: packages,
		}
		return nil
	})

	return nil
}

// DeleteCompose deletes the compose from the state file and also removes all files on disk that are
// associated with this compose
func (s *Store) DeleteCompose(id uuid.UUID) error {
	return s.change(func() error {
		if _, exists := s.composes[id]; !exists {
			return &NotFoundError{}
		}

		delete(s.composes, id)

		return nil
	})
}

// PushSource stores a SourceConfig in store.Sources
func (s *Store) PushSource(key string, source SourceConfig) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		s.sources[key] = source
		return nil
	})
}

// DeleteSourceByName removes a SourceConfig from store.Sources using the .Name field
func (s *Store) DeleteSourceByName(name string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		for key := range s.sources {
			if s.sources[key].Name == name {
				delete(s.sources, key)
				return nil
			}
		}
		return nil
	})
}

// DeleteSourceByID removes a SourceConfig from store.Sources using the ID
func (s *Store) DeleteSourceByID(key string) {
	// FIXME: handle or comment this possible error
	_ = s.change(func() error {
		delete(s.sources, key)
		return nil
	})
}

// ListSourcesByName returns the repo source names
// Name is different than Id, it can be a full description of the repo
func (s *Store) ListSourcesByName() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.sources))
	for _, source := range s.sources {
		names = append(names, source.Name)
	}
	sort.Strings(names)

	return names
}

// ListSourcesById returns the repo source id
// Id is a short identifier for the repo, not a full name description
func (s *Store) ListSourcesById() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.sources))
	for name := range s.sources {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (s *Store) GetSource(name string) *SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source, ok := s.sources[name]
	if !ok {
		return nil
	}
	return &source
}

// GetAllSourcesByName returns the sources using the repo name as the key
func (s *Store) GetAllSourcesByName() map[string]SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make(map[string]SourceConfig)

	for _, v := range s.sources {
		sources[v.Name] = v
	}

	return sources
}

// GetAllSourcesByID returns the sources using the repo id as the key
func (s *Store) GetAllSourcesByID() map[string]SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make(map[string]SourceConfig)

	for k, v := range s.sources {
		sources[k] = v
	}

	return sources
}

// GetAllDistroSources returns the sources using the repo id as the key
// skipping sources that cannot be used with the selected distribution
func (s *Store) GetAllDistroSources(distro string) map[string]SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make(map[string]SourceConfig)

	for k, v := range s.sources {
		found := false
		for _, d := range v.Distros {
			if d == distro {
				found = true
			}
		}
		if len(v.Distros) == 0 || found {
			sources[k] = v
		}
	}

	return sources
}

func NewSourceConfig(repo rpmmd.RepoConfig, system bool) SourceConfig {
	sc := SourceConfig{
		Name:           repo.Name,
		System:         system,
		RHSM:           repo.RHSM,
		GPGKeys:        repo.GPGKeys,
		ModuleHotfixes: repo.ModuleHotfixes,
	}

	if repo.CheckGPG != nil {
		sc.CheckGPG = *repo.CheckGPG
	}

	if repo.CheckRepoGPG != nil {
		sc.CheckRepoGPG = *repo.CheckRepoGPG
	}

	if repo.IgnoreSSL != nil {
		sc.CheckSSL = !*repo.IgnoreSSL
	} else {
		// default should be true to maintain backwards compatibility
		// and current behaviour
		sc.CheckSSL = true
	}

	if len(repo.BaseURLs) != 0 {
		sc.URL = strings.Join(repo.BaseURLs, ",")
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

func (s *SourceConfig) RepoConfig(name string) rpmmd.RepoConfig {
	var repo rpmmd.RepoConfig

	repo.Name = name
	repo.IgnoreSSL = common.ToPtr(!s.CheckSSL)
	repo.CheckGPG = common.ToPtr(s.CheckGPG)
	repo.RHSM = s.RHSM
	repo.CheckRepoGPG = common.ToPtr(s.CheckRepoGPG)
	repo.GPGKeys = s.GPGKeys
	repo.ModuleHotfixes = s.ModuleHotfixes

	var urls []string
	if s.URL != "" {
		urls = []string{s.URL}
	}

	if s.Type == "yum-baseurl" {
		repo.BaseURLs = urls
	} else if s.Type == "yum-metalink" {
		repo.Metalink = s.URL
	} else if s.Type == "yum-mirrorlist" {
		repo.MirrorList = s.URL
	}

	return repo
}
