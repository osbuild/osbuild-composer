package osbuild

// The LocaleStageOptions describes the image's locale.
//
// A locale is typically specified as language_[territory], where language
// is specified in ISO 639 and territory in ISO 3166.
type LocaleStageOptions struct {
	Language string `json:"language"`
}

func (LocaleStageOptions) isStageOptions() {}

// NewLocaleStageOptions creates a new locale stage options object, with
// the mandatory fields set.
func NewLocaleStageOptions(language string) *LocaleStageOptions {
	return &LocaleStageOptions{
		Language: language,
	}
}

// NewLocaleStage creates a new Locale Stage object.
func NewLocaleStage(options *LocaleStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.locale",
		Options: options,
	}
}
