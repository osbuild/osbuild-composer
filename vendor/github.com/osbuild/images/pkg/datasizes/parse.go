package datasizes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Parse converts a size specified as a string in KB/KiB/MB/etc. to
// a number of bytes represented by uint64.
func Parse(size string) (uint64, error) {
	// Pre-process the input
	size = strings.TrimSpace(size)

	// Get the number from the string
	plain_number := regexp.MustCompile(`[[:digit:]]+`)
	number_as_str := plain_number.FindString(size)
	if number_as_str == "" {
		return 0, fmt.Errorf("the size string doesn't contain any number: %s", size)
	}

	// Parse the number into integer
	return_size, err := strconv.ParseUint(number_as_str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size as integer: %s", number_as_str)
	}

	// List of all supported units (from kB to TB and KiB to TiB)
	supported_units := []struct {
		re       *regexp.Regexp
		multiple uint64
	}{
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*kB$`), KiloByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*KiB$`), KibiByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*MB$`), MegaByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*MiB$`), MebiByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*GB$`), GigaByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*GiB$`), GibiByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*TB$`), TeraByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+\s*TiB$`), TebiByte},
		{regexp.MustCompile(`^\s*[[:digit:]]+$`), 1},
	}

	for _, unit := range supported_units {
		if unit.re.MatchString(size) {
			return_size *= unit.multiple
			return return_size, nil
		}
	}

	// In case the strign didn't match any of the above regexes, return nil
	// even if a number was found. This is to prevent users from submitting
	// unknown units.
	return 0, fmt.Errorf("unknown data size units in string: %s", size)
}
