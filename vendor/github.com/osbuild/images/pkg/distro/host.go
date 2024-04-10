package distro

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// variable so that it can be overridden in tests
var getHostDistroNameTree = "/"

// GetHostDistroName returns the name of the host distribution, such as
// "fedora-32" or "rhel-8.2". It does so by reading the /etc/os-release file.
func GetHostDistroName() (string, error) {
	osrelease, err := ReadOSReleaseFromTree(getHostDistroNameTree)
	if err != nil {
		return "", fmt.Errorf("cannot get the host distro name: %w", err)
	}

	if _, ok := osrelease["ID"]; !ok {
		return "", errors.New("cannot get the host distro name: missing ID field in os-release")
	}
	if _, ok := osrelease["VERSION_ID"]; !ok {
		return "", errors.New("cannot get the host distro name: missing VERSION_ID field in os-release")
	}

	name := osrelease["ID"] + "-" + osrelease["VERSION_ID"]

	return name, nil
}

func readOSRelease(r io.Reader) (map[string]string, error) {
	osrelease := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("readOSRelease: invalid input")
		}

		key := strings.TrimSpace(parts[0])
		// drop all surrounding whitespace and double-quotes
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		osrelease[key] = value
	}

	return osrelease, nil
}

// ReadOSReleaseFromTree reads the os-release file from the given root directory.
//
// According to os-release(5), the os-release file should be located in either /etc/os-release or /usr/lib/os-release,
// so both locations are tried, with the former taking precedence.
func ReadOSReleaseFromTree(root string) (map[string]string, error) {
	locations := []string{
		"etc/os-release",
		"usr/lib/os-release",
	}
	var errs []string
	for _, location := range locations {
		f, err := os.Open(path.Join(root, location))
		if err == nil {
			defer f.Close()
			return readOSRelease(f)
		}
		errs = append(errs, fmt.Sprintf("cannot read %s: %v", location, err))
	}

	return nil, fmt.Errorf("failed to read os-release:\n%s", strings.Join(errs, "\n"))
}
