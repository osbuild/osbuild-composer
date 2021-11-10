package osbuild2

type SshdConfigConfig struct {
	PasswordAuthentication          *bool `json:"PasswordAuthentication,omitempty"`
	ChallengeResponseAuthentication *bool `json:"ChallengeResponseAuthentication,omitempty"`
	ClientAliveInterval             *int  `json:"ClientAliveInterval,omitempty"`
}

type SshdConfigStageOptions struct {
	Config SshdConfigConfig `json:"config"`
}

func (SshdConfigStageOptions) isStageOptions() {}

func NewSshdConfigStage(options *SshdConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.sshd.config",
		Options: options,
	}
}
