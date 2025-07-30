package defs

import (
	"fmt"
	"regexp"

	"github.com/osbuild/images/pkg/distro"
)

func validateSubexpMatch(re *regexp.Regexp, names ...string) error {
	for _, name := range names {
		if re.SubexpIndex(name) == -1 {
			return fmt.Errorf("cannot find submatch field %q", name)
		}
	}
	return nil
}

// matchAndNormalize() matches and normalizes the given nameVer
// based on the reStr. On match it returns the normalized version
// of the given nameVer.
func matchAndNormalize(reStr, nameVer string) (string, error) {
	if reStr == "" {
		return "", nil
	}

	re, err := regexp.Compile(`^` + reStr + `$`)
	if err != nil {
		return "", fmt.Errorf("cannot use %q: %w", reStr, err)
	}
	l := re.FindStringSubmatch(nameVer)
	switch len(l) {
	case 0:
		// no match
		return "", nil
	case 1:
		// simple match, no named matching
		return nameVer, nil
	// handling case 2: is not needed, its an incomplete match, we need at least name,major and
	// captured by the "default" below
	case 3:
		// distro only uses major ver and needs normalizing
		if err := validateSubexpMatch(re, "name", "major"); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-%s", l[re.SubexpIndex("name")], l[re.SubexpIndex("major")]), nil
	case 4:
		// common case, major/minor and normalizing
		if err := validateSubexpMatch(re, "name", "major", "minor"); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-%s.%s", l[re.SubexpIndex("name")], l[re.SubexpIndex("major")], l[re.SubexpIndex("minor")]), nil
	default:
		return "", fmt.Errorf("invalid number of submatches for %q %q (%v)", reStr, nameVer, len(l))
	}

}

// ParseID parse the given nameVer into a distro.ID. It will also
// apply normalizations from the distros `match` rule. This is needed
// to support distro names like "rhel-810" without dots.
//
// If no match is found it will "nil" and no error (
func ParseID(nameVer string) (*distro.ID, error) {
	distros, err := loadDistros()
	if err != nil {
		return nil, err
	}

	for _, d := range distros.Distros {
		found, err := matchAndNormalize(d.Match, nameVer)
		if err != nil {
			return nil, err
		}
		if found != "" {
			return distro.ParseID(found)
		}
	}
	return nil, nil
}
