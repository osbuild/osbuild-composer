package main

import (
	"os"
	"path/filepath"
)

type buildResult struct {
	resultGood string
	resultBad  string
}

func newBuildResult(config *Config) *buildResult {
	return &buildResult{
		resultGood: filepath.Join(config.BuildDirBase, "result.good"),
		resultBad:  filepath.Join(config.BuildDirBase, "result.bad"),
	}
}

func (br *buildResult) Mark(err error) error {
	if err == nil {
		return os.WriteFile(br.resultGood, nil, 0600)
	} else {
		return os.WriteFile(br.resultBad, nil, 0600)
	}
}

// todo: switch to (Good, Bad, Unknown)
func (br *buildResult) Good() bool {
	_, err := os.Stat(br.resultGood)
	return err == nil
}

func (br *buildResult) Bad() bool {
	_, err := os.Stat(br.resultBad)
	return err == nil
}
