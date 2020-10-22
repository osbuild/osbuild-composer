package kojiapi

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSplitExtension(t *testing.T) {
	tests := []struct {
		filename  string
		extension string
	}{
		{filename: "image.qcow2", extension: ".qcow2"},
		{filename: "image.tar.gz", extension: ".tar.gz"},
		{filename: "", extension: ""},
		{filename: ".htaccess", extension: ""},
		{filename: ".weirdfile.txt", extension: ".txt"},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			require.Equal(t, tt.extension, splitExtension(tt.filename))
		})
	}
}
