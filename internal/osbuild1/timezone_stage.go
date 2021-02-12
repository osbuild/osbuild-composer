package osbuild1

type TimezoneStageOptions struct {
	Zone string `json:"zone"`
}

func (TimezoneStageOptions) isStageOptions() {}

func NewTimezoneStage(options *TimezoneStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.timezone",
		Options: options,
	}
}
