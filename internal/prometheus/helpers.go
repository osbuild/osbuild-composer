package prometheus

import (
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
)

// this function replaces dynamic parameter segments in
// a route to reduce metric cardinality. i.e. Any value
// passed to `/compose/{composeId}` will be aggregated
// into the path `/compose/-`
func MakeGenericPaths(paths openapi3.Paths) []string {
	var cleaned []string
	r, err := regexp.Compile("{.*?}")
	if err != nil {
		panic(err)
	}
	for path := range paths {
		path = r.ReplaceAllString(path, "-")
		cleaned = append(cleaned, path)
	}
	return cleaned
}
