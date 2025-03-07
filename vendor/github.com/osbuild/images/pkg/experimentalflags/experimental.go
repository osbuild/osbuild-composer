// Package experimentalflags provides functionality for reading
// options defined in an environment variable named
// IMAGE_BUILDER_EXPERIMENTAL.
//
// These functions should be used to determine, in a common way, if
// experimental features should be enabled when using the libarary.
package experimentalflags

import (
	"os"
	"strconv"
	"strings"
)

const envKEY = "IMAGE_BUILDER_EXPERIMENTAL"

func experimentalOptions() map[string]string {
	expMap := map[string]string{}

	env := os.Getenv(envKEY)
	if env == "" {
		return expMap
	}

	for _, s := range strings.Split(env, ",") {
		l := strings.SplitN(s, "=", 2)
		switch len(l) {
		case 1:
			expMap[l[0]] = "true"
		case 2:
			expMap[l[0]] = l[1]
		}
	}

	return expMap
}

// Bool returns true if there is a boolean option with the given
// option name.
//
// Example usage by the user:
//
//	IMAGE_BUILDER_EXPERIMENTAL=skip-foo,skip-bar=1,skip-baz=true
//
// would result in experimetnalflags.Bool("skip-foo") -> true
func Bool(option string) bool {
	expMap := experimentalOptions()
	b, err := strconv.ParseBool(expMap[option])
	if err != nil {
		// not much we can do for invalid inputs, just assume false
		return false
	}
	return b
}

// String returns the user set string for the given experimental feature.
//
// Note that currently no quoting or escaping is supported, so a string
// can (currently) not contain a "," or a "=".
//
// Example usage by the user:
//
//	IMAGE_BUILDER_EXPERIMENTAL=key=value
//
// would result in experimetnalflags.String("key") -> "value"
func String(option string) string {
	expMap := experimentalOptions()
	return expMap[option]
}
