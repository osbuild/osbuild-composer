package clienterrors_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type customErr struct{}

func (ce *customErr) Error() string {
	return "customErr"
}

func TestErrorInterface(t *testing.T) {
	for _, tc := range []struct {
		err         error
		expectedStr string
	}{
		{fmt.Errorf("some error"), "some error"},
		{&customErr{}, "customErr"},
	} {
		wce := clienterrors.WorkerClientError(2, "reason", tc.err)
		assert.Equal(t, fmt.Sprintf("Code: 2, Reason: reason, Details: %s", tc.expectedStr), wce.String())
	}
}

func TestErrorJSONMarshal(t *testing.T) {
	for _, tc := range []struct {
		err         interface{}
		expectedStr string
	}{
		{fmt.Errorf("some-error"), `"some-error"`},
		{[]error{fmt.Errorf("err1"), fmt.Errorf("err2")}, `["err1","err2"]`},
		{"random detail", `"random detail"`},
	} {
		json, err := json.Marshal(clienterrors.WorkerClientError(2, "reason", tc.err))
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf(`{"id":2,"reason":"reason","details":%s}`, tc.expectedStr), string(json))
	}
}

func TestErrorJSONMarshalDetectsNestedErrs(t *testing.T) {
	details := struct {
		Unrelated string
		NestedErr error
		Nested    struct {
			DeepErr error
		}
	}{
		Unrelated: "unrelated",
		NestedErr: fmt.Errorf("some-nested-error"),
		Nested: struct {
			DeepErr error
		}{
			DeepErr: fmt.Errorf("deep-err"),
		},
	}
	_, err := json.Marshal(clienterrors.WorkerClientError(2, "reason", details))
	assert.Equal(t, `json: error calling MarshalJSON for type *clienterrors.Error: found nested error in {Unrelated:unrelated NestedErr:some-nested-error Nested:{DeepErr:deep-err}}: some-nested-error`, err.Error())
}
