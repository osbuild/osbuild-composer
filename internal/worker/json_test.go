package worker

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/common"
)

func TestOSBuildJobResultSuccessful(t *testing.T) {
	type fields struct {
		OSBuildOutput *common.ComposeResult
		Targets       []TargetResult
		GenericError  string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "unsuccessful osbuild",
			fields: struct {
				OSBuildOutput *common.ComposeResult
				Targets       []TargetResult
				GenericError  string
			}{OSBuildOutput: &common.ComposeResult{Success: false}},
			want: false,
		},
		{
			name: "unsuccessful target",
			fields: struct {
				OSBuildOutput *common.ComposeResult
				Targets       []TargetResult
				GenericError  string
			}{OSBuildOutput: &common.ComposeResult{Success: true}, Targets: []TargetResult{{Error: NewTargetError("test")}}},
			want: false,
		},
		{
			name: "non-empty generic error",
			fields: struct {
				OSBuildOutput *common.ComposeResult
				Targets       []TargetResult
				GenericError  string
			}{OSBuildOutput: &common.ComposeResult{Success: true}, GenericError: "test"},
			want: false,
		},
		{
			name: "successful",
			fields: struct {
				OSBuildOutput *common.ComposeResult
				Targets       []TargetResult
				GenericError  string
			}{OSBuildOutput: &common.ComposeResult{Success: true}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &OSBuildJobResult{
				OSBuildOutput: tt.fields.OSBuildOutput,
				Targets:       tt.fields.Targets,
				GenericError:  tt.fields.GenericError,
			}
			require.Equal(t, tt.want, r.Successful())
		})
	}
}
