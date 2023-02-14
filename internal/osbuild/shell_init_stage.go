package osbuild

import (
	"fmt"
	"regexp"
)

const filenameRegex = "^[a-zA-Z0-9\\.\\-_]{1,250}$"
const envVarRegex = "^[A-Z][A-Z0-9_]*$"

type ShellInitStageOptions struct {
	Files map[string]ShellInitFile `json:"files"`
}

func (ShellInitStageOptions) isStageOptions() {}

func (options ShellInitStageOptions) validate() error {
	fre := regexp.MustCompile(filenameRegex)
	vre := regexp.MustCompile(envVarRegex)
	for fname, kvs := range options.Files {
		if !fre.MatchString(fname) {
			return fmt.Errorf("filename %q doesn't conform to schema (%s)", fname, filenameRegex)
		}

		if len(kvs.Env) == 0 {
			return fmt.Errorf("at least one environment variable must be specified for each file")
		}

		for _, kv := range kvs.Env {
			if !vre.MatchString(kv.Key) {
				return fmt.Errorf("variable name %q doesn't conform to schema (%s)", kv.Key, envVarRegex)
			}
		}
	}

	return nil
}

type ShellInitFile struct {
	Env []EnvironmentVariable `json:"env"`
}

type EnvironmentVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewShellInitStage(options *ShellInitStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.shell.init",
		Options: options,
	}
}
