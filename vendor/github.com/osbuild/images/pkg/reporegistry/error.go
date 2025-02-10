package reporegistry

import (
	"fmt"
	"io/fs"
)

// NoReposLoadedError is an error type that is returned when no repositories
// are loaded from the given paths.
type NoReposLoadedError struct {
	Paths []string
	FSes  []fs.FS
}

func (e *NoReposLoadedError) Error() string {
	return fmt.Sprintf("no repositories found in the given paths: %v/%v", e.Paths, e.FSes)
}
