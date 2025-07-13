package defs

// this file contains legacy code and it can be removed once
// all of rhel is converted to the "generic" distro

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/rpmmd"
)

// XXX: compat only, will go away once we move to "generic" distros
// everywhere
func PackageSets(it distro.ImageType) (map[string]rpmmd.PackageSet, error) {
	archName := it.Arch().Name()
	distroNameVer := it.Arch().Distro().Name()

	// each imagetype can have multiple package sets, so that we can
	// use yaml aliases/anchors to de-duplicate them
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	imgType, ok := toplevel.ImageTypes[it.Name()]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, it.Name())
	}
	return imgType.PackageSets(distroNameVer, archName)
}

// XXX: compat only, will go away once we move to "generic" distros
// everywhere
func PartitionTable(it distro.ImageType) (*disk.PartitionTable, error) {
	distroNameVer := it.Arch().Distro().Name()

	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}

	imgType, ok := toplevel.ImageTypes[it.Name()]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, it.Name())
	}
	return imgType.PartitionTable(distroNameVer, it.Arch().Name())
}

// XXX: compat only, will go away once we move to "generic" distros
// everywhere
func DistroImageConfig(distroNameVer string) (*distro.ImageConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	return toplevel.ImageConfig.For(distroNameVer)
}

// XXX: compat only, will go away once we move to "generic" distros
// everywhere
func ImageConfig(distroNameVer, archName, typeName string) (*distro.ImageConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	return imgType.ImageConfig(distroNameVer, archName)
}

// XXX: compat only, will go away once we move to "generic" distros
// everywhere
func InstallerConfig(distroNameVer, archName, typeName string) (*distro.InstallerConfig, error) {
	toplevel, err := load(distroNameVer)
	if err != nil {
		return nil, err
	}
	imgType, ok := toplevel.ImageTypes[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrImageTypeNotFound, typeName)
	}
	imgType.name = typeName

	return imgType.InstallerConfig(distroNameVer, archName)
}

// Cache the toplevel structure, loading/parsing YAML is quite
// expensive. This can all be removed in the future where there
// is a single load for each distroNameVer. Right now the various
// helpers (like ParititonTable(), ImageConfig() are called a
// gazillion times. However once we move into the "generic" distro
// the distro will do a single load/parse of all image types and
// just reuse them and this can go.
type imageTypesCache struct {
	cache map[string]*imageTypesYAML
	mu    sync.Mutex
}

func newImageTypesCache() *imageTypesCache {
	return &imageTypesCache{cache: make(map[string]*imageTypesYAML)}
}

func (i *imageTypesCache) Get(hash string) *imageTypesYAML {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.cache[hash]
}

func (i *imageTypesCache) Set(hash string, ity *imageTypesYAML) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cache[hash] = ity
}

var (
	itCache = newImageTypesCache()
)

func load(distroNameVer string) (*imageTypesYAML, error) {
	id, err := distro.ParseID(distroNameVer)
	if err != nil {
		return nil, err
	}

	// XXX: this is only needed temporary until we have a "distros.yaml"
	// that describes some high-level properties of each distro
	// (like their yaml dirs)
	var baseDir string
	switch id.Name {
	case "rhel", "almalinux", "centos", "almalinux_kitten":
		// rhel yaml files are under ./rhel-$majorVer
		// almalinux yaml is just rhel, we take only its major version
		// centos and kitten yaml is just rhel but we have (sadly) no
		// symlinks in "go:embed" so we have to have this slightly ugly
		// workaround
		baseDir = fmt.Sprintf("rhel-%v", id.MajorVersion)
	case "test-distro":
		// our other distros just have a single yaml dir per distro
		// and use condition.version_gt etc
		baseDir = id.Name
	}

	// take the base path from the distros.yaml
	distro, err := NewDistroYAML(distroNameVer)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if distro != nil && distro.DefsPath != "" {
		baseDir = distro.DefsPath
	}

	f, err := dataFS().Open(filepath.Join(baseDir, "distro.yaml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// XXX: this is currently needed because rhel distros call
	// ImageType() and ParitionTable() a gazillion times and
	// each time the full yaml is loaded. Once things move to
	// the "generic" distro this will no longer be the case and
	// this cache can be removed and below we can decode directly
	// from "f" again instead of wasting memory with "buf"
	var buf bytes.Buffer
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(&buf, h), f); err != nil {
		return nil, fmt.Errorf("cannot read from %s: %w", baseDir, err)
	}
	inputHash := string(h.Sum(nil))
	if cached := itCache.Get(inputHash); cached != nil {
		return cached, nil
	}

	var toplevel imageTypesYAML
	decoder := yaml.NewDecoder(&buf)
	decoder.KnownFields(true)
	if err := decoder.Decode(&toplevel); err != nil {
		return nil, err
	}

	// XXX: remove once we no longer need caching
	itCache.Set(inputHash, &toplevel)

	return &toplevel, nil
}
