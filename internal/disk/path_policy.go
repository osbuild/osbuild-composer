package disk

import (
	"fmt"
	"path"
)

type PathPolicy struct {
	Deny  bool // explicitly do not allow this entry
	Exact bool // require and exact match, no subdirs
}

type PathPolicies = PathTrie

// Create a new PathPolicies trie from a map of path to PathPolicy
func NewPathPolicies(entries map[string]PathPolicy) *PathPolicies {

	noType := make(map[string]interface{}, len(entries))

	for k, v := range entries {
		noType[k] = v
	}

	return NewPathTrieFromMap(noType)
}

// Check a given path at dir against the PathPolicies
func (pol *PathPolicies) Check(dir string) error {

	// Quickly check we have a mountpoint and it is absolute
	if dir == "" || dir[0] != '/' {
		return fmt.Errorf("mountpoint must be absolute path")
	}

	// ensure that only clean mountpoints are valid
	if dir != path.Clean(dir) {
		return fmt.Errorf("mountpoint must be a canonical path")
	}

	node, left := pol.Lookup(dir)
	policy, ok := node.Payload.(PathPolicy)
	if !ok {
		panic("programming error: invalid path trie payload")
	}

	// 1) path is explicitly not allowed or
	// 2) a subpath was match but an explicit match is required
	if policy.Deny || (policy.Exact && len(left) > 0) {
		return fmt.Errorf("path '%s ' is not allowed", dir)
	}

	// exact match or recursive mountpoints allowed
	return nil
}
