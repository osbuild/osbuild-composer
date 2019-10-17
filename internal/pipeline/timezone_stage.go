package pipeline

type TimezoneStageOptions struct {
	Zone string `json:"zone"`
}

func (TimezoneStageOptions) isStageOptions() {}

func NewTimezoneStage(options *TimezoneStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.users",
		Options: options,
	}
}
