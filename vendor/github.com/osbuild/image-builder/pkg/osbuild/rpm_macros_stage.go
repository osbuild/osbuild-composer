package osbuild

type RPMMacrosStageOptions struct {
	Filename string    `json:"filename" yaml:"filename"`
	Macros   RPMMacros `json:"macros" yaml:"macros"`
}

type RPMMacros struct {
	InstallLangs []string `json:"_install_langs,omitempty" yaml:"_install_langs,omitempty"`
	DBPath       string   `json:"_dbpath,omitempty" yaml:"_dbpath,omitempty"`
}

func (RPMMacrosStageOptions) isStageOptions() {}

func NewRPMMacrosStage(options *RPMMacrosStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm.macros",
		Options: options,
	}
}
