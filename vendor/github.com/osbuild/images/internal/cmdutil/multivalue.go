package cmdutil

import (
	"strings"

	"github.com/gobwas/glob"
)

// MultiValue can be used as a flag.Var type to support comma-separated lists
// of values on the command line. The [MultiValue.ResolveArgValues] method can
// be used to validate the values and resolve globs.
type MultiValue []string

func (mv *MultiValue) String() string {
	return strings.Join(*mv, ", ")
}

func (mv *MultiValue) Set(v string) error {
	*mv = strings.Split(v, ",")
	return nil
}

// ResolveArgValues returns a list of valid values from the MultiValue. Invalid
// values are returned separately. Globs are expanded. If the args are empty,
// the valueList is returned in full.
func (args MultiValue) ResolveArgValues(valueList []string) ([]string, []string) {
	if len(args) == 0 {
		return valueList, nil
	}
	selection := make([]string, 0, len(args))
	invalid := make([]string, 0, len(args))
	for _, arg := range args {
		g := glob.MustCompile(arg)
		match := false
		for _, v := range valueList {
			if g.Match(v) {
				selection = append(selection, v)
				match = true
			}
		}
		if !match {
			invalid = append(invalid, arg)
		}
	}
	return selection, invalid
}
