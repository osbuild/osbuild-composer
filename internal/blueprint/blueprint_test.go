package blueprint_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
)

func TestInvalidOutputFormatError_Error(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "basic",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &blueprint.InvalidOutputFormatError{}
			if got := e.Error(); got != tt.want {
				t.Errorf("InvalidOutputFormatError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
