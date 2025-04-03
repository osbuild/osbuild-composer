package disk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
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

func unmarshalJSONPayload(data []byte) (PayloadEntity, error) {
	var payload struct {
		Payload     json.RawMessage `json:"payload"`
		PayloadType string          `json:"payload_type,omitempty"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("cannot peek payload: %w", err)
	}
	if payload.PayloadType == "" {
		if len(payload.Payload) > 0 {
			return nil, fmt.Errorf("cannot build payload: empty payload type but payload is: %q", payload.Payload)
		}
		return nil, nil
	}
	entType := payloadEntityMap[payload.PayloadType]
	if entType == nil {
		return nil, fmt.Errorf("cannot build payload from %q: unknown payload type %q", data, payload.PayloadType)
	}
	entValP := reflect.New(entType).Elem().Addr()
	ent := entValP.Interface()
	if err := jsonUnmarshalStrict(payload.Payload, &ent); err != nil {
		return nil, fmt.Errorf("cannot decode payload for %q: %w", data, err)
	}
	return ent.(PayloadEntity), nil
}

func jsonUnmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewBuffer(data))
	dec.DisallowUnknownFields()
	return dec.Decode(&v)
}
