package distrodefs

import "embed"

//go:embed *.yaml */*.yaml
var Data embed.FS
