package blueprint

import (
	"encoding/json"
	"fmt"
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
