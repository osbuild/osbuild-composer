package oci

// KEEP IN SYNC:
// this is a copy of osbuild/oci_archive_stage.go:OCIArchiveConfig
// with nicer names
type OCIArchiveConfig struct {
	Cmd          []string          `yaml:"cmd,omitempty"`
	Env          []string          `yaml:"env,omitempty"`
	ExposedPorts []string          `yaml:"exposed_ports,omitempty"`
	User         string            `yaml:"user,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"`
	StopSignal   string            `yaml:"stop_signal,omitempty"`
	Volumes      []string          `yaml:"volumes,omitempty"`
	WorkingDir   string            `yaml:"working_dir,omitempty"`
}

type OCI struct {
	Archive *OCIArchiveConfig `yaml:"archive,omitempty"`
}
