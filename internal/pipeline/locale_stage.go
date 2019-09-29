package pipeline

type LocaleStageOptions struct {
	Language string `json:"language"`
}

func (LocaleStageOptions) isStageOptions() {}

func NewLocaleStageOptions(language string) *LocaleStageOptions {
	return &LocaleStageOptions{
		Language: language,
	}
}

func NewLocaleStage(options *LocaleStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.locale",
		Options: options,
	}
}
