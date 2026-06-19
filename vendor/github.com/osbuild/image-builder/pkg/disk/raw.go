package disk

import (
	"reflect"
)

// Raw defines the payload for a raw partition. It's similar to a
// [Filesystem] but with fewer fields. It is a [PayloadEntity].
type Raw struct {
	SourcePath string `json:"source_path" yaml:"source_path"`
}

func init() {
	payloadEntityMap["raw"] = reflect.TypeOf(Raw{})
}

func (s *Raw) EntityName() string {
	return "raw"
}

func (s *Raw) Clone() Entity {
	if s == nil {
		return nil
	}

	return &Raw{
		SourcePath: s.SourcePath,
	}
}
