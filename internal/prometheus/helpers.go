package prometheus

import (
	"regexp"
	"strings"
	"time"
)

type ObserveFunc func() time.Duration

func pathLabel(path string) string {
	r := regexp.MustCompile(":(.*)")
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = r.ReplaceAllString(segment, "-")
	}
	return strings.Join(segments, "/")
}
