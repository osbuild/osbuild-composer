package reporegistry

import "fmt"

// NoReposLoadedError is an error type that is returned when no repositories
// are loaded from the given paths.
type NoReposLoadedError struct {
	Paths []string
}

func (e *NoReposLoadedError) Error() string {
	return fmt.Sprintf("no repositories found in the given paths: %v", e.Paths)
}
