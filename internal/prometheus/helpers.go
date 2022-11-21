package prometheus

import (
	"regexp"
	"strings"
)

func pathLabel(path string) string {
	r := regexp.MustCompile(":(.*)")
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = r.ReplaceAllString(segment, "-")
	}
	return strings.Join(segments, "/")
}
