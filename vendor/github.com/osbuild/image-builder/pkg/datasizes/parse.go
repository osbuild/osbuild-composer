package datasizes

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	plainNumberRegexp = regexp.MustCompile(`^(\d+(\.\d+)?)`)

	// List of all supported units (from kB to TB and KiB to TiB)
	supportedUnitsRegexp = []struct {
		re       *regexp.Regexp
		multiple uint64
	}{
		{regexp.MustCompile(`^\d+(\.\d+)?\s*kB$`), KiloByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*KiB$`), KibiByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*MB$`), MegaByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*MiB$`), MebiByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*GB$`), GigaByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*GiB$`), GibiByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*TB$`), TeraByte},
		{regexp.MustCompile(`^\d+(\.\d+)?\s*TiB$`), TebiByte},
		{regexp.MustCompile(`^\d+(\.\d+)?$`), 1},
	}
)

// Parse converts a size specified as a string in KB/KiB/MB/etc. to
// a number of bytes represented by uint64.
// Floats are allowed for units other than bytes (e.g., "1.5 GiB" converts to bytes).
// For bytes (no unit), fractional values like "1.5" or "123.45 B" error out.
func Parse(size string) (uint64, error) {
	// Pre-process the input
	size = strings.TrimSpace(size)

	// Get the number from the string
	numberAsStr := plainNumberRegexp.FindString(size)
	if numberAsStr == "" {
		return 0, fmt.Errorf("the size string is not a valid positive float number: %s", size)
	}

	// Parse the number
	numberFloat, err := strconv.ParseFloat(numberAsStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size as float: %s", numberAsStr)
	}

	for _, unit := range supportedUnitsRegexp {
		if unit.re.MatchString(size) {
			sizeInBytes := numberFloat * float64(unit.multiple)
			sizeInBytesInt := uint64(math.Round(sizeInBytes))
			return sizeInBytesInt, nil
		}
	}

	// In case the string didn't match any of the above regexes, return nil
	// even if a number was found. This is to prevent users from submitting
	// unknown units.
	return 0, fmt.Errorf("unknown data size units in string: %s", size)
}
