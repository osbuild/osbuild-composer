package osbuild

import (
	"crypto/sha256"
	"fmt"
)

type FDOStageReferences []string

func (FDOStageReferences) isReferences() {}

type FDOStageInput struct {
	inputCommon
	References FDOStageReferences `json:"references"`
}

func (FDOStageInput) isStageInput() {}

type FDOStageInputs struct {
	RootCerts *FDOStageInput `json:"rootcerts"`
}

func (FDOStageInputs) isStageInputs() {}

// NewFDOStageForCert creates FDOStage
func NewFDOStageForRootCerts(rootCertsData string) *Stage {

	dataBytes := []byte(rootCertsData)
	rootCertsInputHash := fmt.Sprintf("sha256:%x", sha256.Sum256(dataBytes))

	input := new(FDOStageInput)
	input.Type = "org.osbuild.files"
	input.Origin = "org.osbuild.source"
	input.References = FDOStageReferences{rootCertsInputHash}

	return &Stage{
		Type:   "org.osbuild.fdo",
		Inputs: &FDOStageInputs{RootCerts: input},
	}
}
