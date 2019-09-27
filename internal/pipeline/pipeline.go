package pipeline

import (
	"encoding/json"
	"errors"
)

type Pipeline struct {
	BuildPipeline *Pipeline  `json:"build,omitempty"`
	Stages        []*Stage   `json:"stages,omitempty"`
	Assembler     *Assembler `json:"assembler,omitempty"`
}

type Stage struct {
	Name    string       `json:"name"`
	Options StageOptions `json:"options"`
}

type StageOptions interface {
	isStageOptions()
}

type rawStage struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
}

func (stage *Stage) UnmarshalJSON(data []byte) error {
	var rawStage rawStage
	err := json.Unmarshal(data, &rawStage)
	if err != nil {
		return err
	}
	var options StageOptions
	switch rawStage.Name {
	case "org.osbuild.dnf":
		options = new(DNFStageOptions)
	case "org.osbuild.fix-bls":
		options = new(FixBLSStageOptions)
	case "org.osbuild.FSTab":
		options = new(FSTabStageOptions)
	case "org.osbuild.grub2":
		options = new(GRUB2StageOptions)
	case "org.osbuild.locale":
		options = new(LocaleStageOptions)
	case "org.osbuild.SELinux":
		options = new(SELinuxStageOptions)
	default:
		return errors.New("unexpected stage name")
	}
	err = json.Unmarshal(rawStage.Options, options)
	if err != nil {
		return err
	}

	stage.Name = rawStage.Name
	stage.Options = options

	return nil
}

type Assembler struct {
	Name    string           `json:"name"`
	Options AssemblerOptions `json:"options"`
}
type AssemblerOptions interface {
	isAssemblerOptions()
}

type rawAssembler struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
}

func (assembler *Assembler) UnmarshalJSON(data []byte) error {
	var rawAssembler rawAssembler
	err := json.Unmarshal(data, &rawAssembler)
	if err != nil {
		return err
	}
	var options AssemblerOptions
	switch rawAssembler.Name {
	case "org.osbuild.tar":
		options = new(TarAssemblerOptions)
	case "org.osbuild.qcow2":
		options = new(QCOW2AssemblerOptions)
	default:
		return errors.New("unexpected assembler name")
	}
	err = json.Unmarshal(rawAssembler.Options, options)
	if err != nil {
		return err
	}

	assembler.Name = rawAssembler.Name
	assembler.Options = options

	return nil
}

func (p *Pipeline) SetBuildPipeline(buildPipeline *Pipeline) {
	p.BuildPipeline = buildPipeline
}

func (p *Pipeline) AddStage(stage *Stage) {
	p.Stages = append(p.Stages, stage)
}

func (p *Pipeline) SetAssembler(assembler *Assembler) {
	p.Assembler = assembler
}
