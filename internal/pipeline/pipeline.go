package pipeline

type Pipeline struct {
	Stages    []Stage   `json:"stages,omitempty"`
	Assembler Assembler `json:"assembler"`
}

type Stage struct {
}

type Assembler struct {
	Name    string              `json:"name"`
	Options AssemblerTarOptions `json:"options"`
}

type AssemblerTarOptions struct {
	Filename string `json:"filename"`
}
