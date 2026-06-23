package flatpak

import (
	"fmt"
	"strings"
)

type Reference struct {
	Type       string
	Identifier string
	Arch       string
	Branch     string
}

func NewReferenceFromString(ref string) (Reference, error) {
	parts := strings.Split(ref, "/")

	if len(parts) != 4 {
		return Reference{}, fmt.Errorf("could not parse ref, got %d parts", len(parts))
	}

	// make sure all parts are non-empty
	for index, part := range parts {
		if len(part) == 0 {
			return Reference{}, fmt.Errorf("could not parse ref, got empty part at index %d", index)
		}
	}

	return Reference{
		Type:       parts[0],
		Identifier: parts[1],
		Arch:       parts[2],
		Branch:     parts[3],
	}, nil
}

func (r *Reference) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.Type, r.Identifier, r.Arch, r.Branch)
}
