package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func GetHostDistroName() (string, bool, bool, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", false, false, err
	}
	defer f.Close()
	osrelease, err := readOSRelease(f)
	if err != nil {
		return "", false, false, err
	}

	isStream := osrelease["NAME"] == "CentOS Stream"

	version := strings.Split(osrelease["VERSION_ID"], ".")
	name := osrelease["ID"] + "-" + strings.Join(version, "")

	// TODO: We should probably index these things by the full CPE
	beta := strings.Contains(osrelease["CPE_NAME"], "beta")
	return name, beta, isStream, nil
}

// GetRedHatRelease returns the content of /etc/redhat-release
// without the trailing new-line.
func GetRedHatRelease() (string, error) {
	raw, err := ioutil.ReadFile("/etc/redhat-release")
	if err != nil {
		return "", fmt.Errorf("cannot read /etc/redhat-release: %v", err)
	}

	//Remove the trailing new-line.
	redHatRelease := strings.TrimSpace(string(raw))

	return redHatRelease, nil
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
