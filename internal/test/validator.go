package test

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-cmp/cmp"
)

// BodyValidator is an abstract interface for defining validators for response bodies
type BodyValidator interface {
	// Validate returns nil if the body is valid. If the body isn't valid, a descriptive error is returned.
	Validate(body []byte) error
}

// JSONValidator is a simple validator for validating JSON responses
type JSONValidator struct {
	// Content is the expected json content of the body
	//
	// Note that the key order of maps is arbitrary
	Content string

	// IgnoreFields is a list of JSON keys that should be removed from both expected body and actual body
	IgnoreFields []string
}

func (b JSONValidator) Validate(body []byte) error {
	var reply, expected interface{}
	err := json.Unmarshal(body, &reply)
	if err != nil {
		return fmt.Errorf("json.Unmarshal failed: %s\n%w", string(body), err)
	}

	err = json.Unmarshal([]byte(b.Content), &expected)
	if err != nil {
		return fmt.Errorf("expected JSON is invalid: %s\n%w", string(b.Content), err)
	}

	dropFields(reply, b.IgnoreFields...)
	dropFields(expected, b.IgnoreFields...)

	diff := cmp.Diff(expected, reply)
	if diff != "" {
		return fmt.Errorf("bodies don't match: %s", diff)
	}

	return nil
}
