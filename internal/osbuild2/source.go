package osbuild2

import (
	"encoding/json"
	"errors"
)

// A Sources map contains all the sources made available to an osbuild run
type Sources map[string]Source

// Source specifies the operations of a given source-type.
type Source interface {
	isSource()
}

type SourceOptions interface {
	isSourceOptions()
}

type rawSources map[string]json.RawMessage

// UnmarshalJSON unmarshals JSON into a Source object. Each type of source has
// a custom unmarshaller for its options, selected based on the source name.
func (sources *Sources) UnmarshalJSON(data []byte) error {
	var rawSources rawSources
	err := json.Unmarshal(data, &rawSources)
	if err != nil {
		return err
	}
	*sources = make(map[string]Source)
	for name, rawSource := range rawSources {
		var source Source
		switch name {
		case "org.osbuild.curl":
			source = new(CurlSource)
		case "org.osbuild.ostree":
			source = new(OSTreeSource)
		default:
			return errors.New("unexpected source name: " + name)
		}
		err = json.Unmarshal(rawSource, source)
		if err != nil {
			return err
		}
		(*sources)[name] = source
	}

	return nil
}
