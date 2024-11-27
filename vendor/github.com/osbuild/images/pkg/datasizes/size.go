package datasizes

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Size is a wrapper around uint64 with support for reading from string
// yaml/toml, so {"size": 123}, {"size": "1234"}, {"size": "1 GiB"} are
// all supported
type Size uint64

// Uint64 returns the size as uint64. This is a convenience functions,
// it is strictly equivalent to uint64(Size(1))
func (si Size) Uint64() uint64 {
	return uint64(si)
}

func (si *Size) UnmarshalTOML(data interface{}) error {
	i, err := decodeSize(data)
	if err != nil {
		return fmt.Errorf("error decoding TOML size: %w", err)
	}
	*si = Size(i)
	return nil
}

func (si *Size) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewBuffer(data))
	dec.UseNumber()

	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return err
	}
	i, err := decodeSize(v)
	if err != nil {
		// if only we could do better here and include e.g. the field
		// name where this happend but encoding/json does not
		// support this, c.f. https://github.com/golang/go/issues/58655
		return fmt.Errorf("error decoding size: %w", err)
	}
	*si = Size(i)
	return nil
}

// decodeSize takes an integer or string representing a data size (with a data
// suffix) and returns the uint64 representation.
func decodeSize(size any) (uint64, error) {
	switch s := size.(type) {
	case string:
		return Parse(s)
	case json.Number:
		i, err := s.Int64()
		if i < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		return uint64(i), err
	case int64:
		if s < 0 {
			return 0, fmt.Errorf("cannot be negative")
		}
		return uint64(s), nil
	case uint64:
		return s, nil
	case float64, float32:
		return 0, fmt.Errorf("cannot be float")
	default:
		return 0, fmt.Errorf("failed to convert value \"%v\" to number", size)
	}
}
