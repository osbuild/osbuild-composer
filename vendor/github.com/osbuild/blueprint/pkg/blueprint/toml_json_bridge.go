package blueprint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"
)

// XXX: move to interal/common ?
func unmarshalTOMLviaJSON(u json.Unmarshaler, data any) error {
	// This is the most efficient way to reuse code when unmarshaling
	// structs in toml, it leaks json errors which is a bit sad but
	// because the toml unmarshaler gives us not "[]byte" but an
	// already pre-processed "any" we cannot just unmarshal into our
	// "fooMarshaling" struct and reuse the result so we resort to
	// this workaround (but toml will go away long term anyway).
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error unmarshaling TOML data %v: %w", data, err)
	}
	if err := u.UnmarshalJSON(dataJSON); err != nil {
		return fmt.Errorf("error decoding TOML %v: %w", data, err)
	}
	return nil
}

// jsonToToml converts a JSON byte slice to a TOML byte slice.
func jsonToToml(data []byte) ([]byte, error) {
	var result any

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(result); err != nil {
		return nil, fmt.Errorf("error marshaling to TOML: %w", err)
	}

	return buf.Bytes(), nil
}

// tomlEq compares two TOML byte slices for equality
func tomlEq(expected []byte, actual []byte) (bool, error) {
	var expectedMap, actualMap map[string]any

	if err := toml.Unmarshal(expected, &expectedMap); err != nil {
		return false, fmt.Errorf("error unmarshaling expected TOML: %w", err)
	}
	if err := toml.Unmarshal(actual, &actualMap); err != nil {
		return false, fmt.Errorf("error unmarshaling actual TOML: %w", err)
	}

	return reflect.DeepEqual(expectedMap, actualMap), nil
}
