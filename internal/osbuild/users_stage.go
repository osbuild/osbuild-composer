package osbuild

type UsersStageOptions struct {
	Users map[string]UsersStageOptionsUser `json:"users"`
}

func (UsersStageOptions) isStageOptions() {}

type UsersStageOptionsUser struct {
	UID         *string  `json:"uid,omitempty"`
	GID         *string  `json:"gid,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Description *string  `json:"description,omitempty"`
	Home        *string  `json:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty"`
	Password    *string  `json:"password,omitempty"`
	Key         *string  `json:"key,omitempty"`
}

func NewUsersStage(options *UsersStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.users",
		Options: options,
	}
}
