package rpmmd

type ModuleSpec struct {
	ModuleConfigFile ModuleConfigFile   `json:"module-file"`
	FailsafeFile     ModuleFailsafeFile `json:"failsafe-file"`
}

type ModuleConfigFile struct {
	Path string           `json:"path"`
	Data ModuleConfigData `json:"data"`
}

type ModuleConfigData struct {
	Name     string   `json:"name"`
	Stream   string   `json:"stream"`
	Profiles []string `json:"profiles"`
	State    string   `json:"state"`
}

type ModuleFailsafeFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}
