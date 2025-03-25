package disk

import (
	"encoding/json"
	"fmt"
)

// unmarshalYAMLviaJSON unmarshals via the JSON interface, this avoids code
// duplication on the expense of slightly uglier errors
func unmarshalYAMLviaJSON(u json.Unmarshaler, unmarshal func(any) error) error {
	var data any
	if err := unmarshal(&data); err != nil {
		return fmt.Errorf("cannot unmarshal to any: %w", err)
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("unmarshal yaml via json failed: %w", err)
	}
	if err := u.UnmarshalJSON(dataJSON); err != nil {
		return fmt.Errorf("unmarshal yaml via json for %s failed: %w", dataJSON, err)
	}
	return nil
}
