package common

import (
	"regexp"
	"runtime"
	"sort"
	"strconv"
)

var RuntimeGOARCH = runtime.GOARCH

func CurrentArch() string {
	if RuntimeGOARCH == "amd64" {
		return "x86_64"
	} else if RuntimeGOARCH == "arm64" {
		return "aarch64"
	} else if RuntimeGOARCH == "ppc64le" {
		return "ppc64le"
	} else if RuntimeGOARCH == "s390x" {
		return "s390x"
	} else {
		panic("unsupported architecture")
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// IsStringInSortedSlice returns true if the string is present, false if not
// slice must be sorted
func IsStringInSortedSlice(slice []string, s string) bool {
	i := sort.SearchStrings(slice, s)
	if i < len(slice) && slice[i] == s {
		return true
	}
	return false
}

func FsSizeToUint64(size string) *uint64 {
	// Read the number in the string
	plain_number := regexp.MustCompile(`[[:digit:]]+`)
	numbers_as_str := plain_number.FindAllString(size, 1)
	number, err := strconv.ParseInt(numbers_as_str[0], 10, 64)
	if err != nil {
		return nil
	}
	return_size := uint64(number)

	mega_byte := regexp.MustCompile(`[[:digit:]]+\s*(m|M)(b|B)?`)
	giga_byte := regexp.MustCompile(`[[:digit:]]+\s*(g|G)(b|B)?`)

	if plain_number.MatchString(size) {
		return &return_size
	}
	if mega_byte.MatchString(size) {
		return_size *= 1000000
		return &return_size
	}
	if giga_byte.MatchString(size) {
		return_size *= 1000000000
		return &return_size
	}

	return nil
}
