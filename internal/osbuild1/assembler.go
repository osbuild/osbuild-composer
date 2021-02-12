package osbuild1

import (
	"encoding/json"
	"errors"
)

// An Assembler turns a filesystem tree into a target image.
type Assembler struct {
	Name    string           `json:"name"`
	Options AssemblerOptions `json:"options"`
}

// AssemblerOptions specify the operations of a given assembler-type.
type AssemblerOptions interface {
	isAssemblerOptions()
}

type rawAssembler struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
}

// UnmarshalJSON unmarshals JSON into an Assembler object. Each type of
// assembler has a custom unmarshaller for its options, selected based on the
// stage name.
func (assembler *Assembler) UnmarshalJSON(data []byte) error {
	var rawAssembler rawAssembler
	err := json.Unmarshal(data, &rawAssembler)
	if err != nil {
		return err
	}
	var options AssemblerOptions
	switch rawAssembler.Name {
	case "org.osbuild.ostree.commit":
		options = new(OSTreeCommitAssemblerOptions)
	case "org.osbuild.qemu":
		options = new(QEMUAssemblerOptions)
	case "org.osbuild.rawfs":
		options = new(RawFSAssemblerOptions)
	case "org.osbuild.tar":
		options = new(TarAssemblerOptions)
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
