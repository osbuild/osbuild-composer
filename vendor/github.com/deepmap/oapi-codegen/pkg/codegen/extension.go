package codegen

import (
	"encoding/json"
	"fmt"
)

const (
	extPropGoType    = "x-go-type"
	extPropOmitEmpty = "x-omitempty"
	extPropExtraTags = "x-oapi-codegen-extra-tags"
)

func extTypeName(extPropValue interface{}) (string, error) {
	raw, ok := extPropValue.(json.RawMessage)
	if !ok {
		return "", fmt.Errorf("failed to convert type: %T", extPropValue)
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return name, nil
}

func extParseOmitEmpty(extPropValue interface{}) (bool, error) {
	raw, ok := extPropValue.(json.RawMessage)
	if !ok {
		return false, fmt.Errorf("failed to convert type: %T", extPropValue)
	}

	var omitEmpty bool
	if err := json.Unmarshal(raw, &omitEmpty); err != nil {
		return false, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return omitEmpty, nil
}

func extExtraTags(extPropValue interface{}) (map[string]string, error) {
	raw, ok := extPropValue.(json.RawMessage)
	if !ok {
		return nil, fmt.Errorf("failed to convert type: %T", extPropValue)
	}
	var tags map[string]string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}
	return tags, nil
}
