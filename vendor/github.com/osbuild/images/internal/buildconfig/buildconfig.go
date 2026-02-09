package buildconfig

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/rpmmd"
)

type BuildConfig struct {
	Name      string               `json:"name"`
	Blueprint *blueprint.Blueprint `json:"blueprint,omitempty"`
	Options   distro.ImageOptions  `json:"options"`
	Depends   interface{}          `json:"depends,omitempty"` // ignored
	Solver    *SolverConfig        `json:"solver,omitempty"`

	// CustomRepos is a list of additional repositories that will be used for
	// depsolving the image package sets.
	CustomRepos []rpmmd.RepoConfig `json:"custom_repos,omitempty"`
}

// SolverConfig is a configuration for the depsolver used by gen-manifests.
// This is added specifically to workaround how bib depsolves package sets
// for the "legacy anaconda ISO" image type, where it sets the root directory
// to point to the root of a mounted bootc container image, instead of
// setting the Repositories in the package sets.
type SolverConfig struct {
	UseRootDir bool `json:"use_root_dir,omitempty"`
}

type Options struct {
	AllowUnknownFields bool
}

func New(path string, opts *Options) (*BuildConfig, error) {
	if opts == nil {
		opts = &Options{}
	}

	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dec := json.NewDecoder(fp)
	if !opts.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}
	var conf BuildConfig

	if err := dec.Decode(&conf); err != nil {
		return nil, fmt.Errorf("cannot decode build config: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("multiple configuration objects or extra data found in %q", path)
	}
	return &conf, nil
}
