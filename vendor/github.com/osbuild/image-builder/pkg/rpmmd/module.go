package rpmmd

type ModuleSpec struct {
	ModuleConfigFile ModuleConfigFile
	FailsafeFile     ModuleFailsafeFile
}

type ModuleConfigFile struct {
	Path string
	Data ModuleConfigData
}

type ModuleConfigData struct {
	Name     string
	Stream   string
	Profiles []string
	State    string
}

type ModuleFailsafeFile struct {
	Path string
	Data string
}
