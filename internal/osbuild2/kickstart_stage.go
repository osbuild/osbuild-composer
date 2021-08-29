package osbuild2

type KickstartStageOptions struct {
	// Where to place the kickstart file
	Path string `json:"path"`

	OSTree *OSTreeOptions `json:"ostree,omitempty"`

	LiveIMG *LiveIMG `json:"liveimg,omitempty"`

	Users  map[string]UsersStageOptionsUser   `json:"users,omitempty"`
	Groups map[string]GroupsStageOptionsGroup `json:"groups,omitempty"`
}

type LiveIMG struct {
	URL string `json:"url"`
}

type OSTreeOptions struct {
	OSName string `json:"osname"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	GPG    bool   `json:"gpg"`
}

func (KickstartStageOptions) isStageOptions() {}

// Creates an Anaconda kickstart file
func NewKickstartStage(options *KickstartStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.kickstart",
		Options: options,
	}
}
