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

// GetHostDistroName returns the name and major version of the host
// distribution in the form <name>-<major version> (e.g., rhel-8, centos-9,
// fedora-42)
func GetHostDistroName() (string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer f.Close()
	osrelease, err := readOSRelease(f)
	if err != nil {
		return "", err
	}

	version := strings.Split(osrelease["VERSION_ID"], ".")
	name := osrelease["ID"] + "-" + version[0]
	return name, nil
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
		value := strings.TrimSpace(parts[1])
		if value[0] == '"' {
			if len(value) < 2 || value[len(value)-1] != '"' {
				return nil, errors.New("readOSRelease: invalid input")
			}
			value = value[1 : len(value)-1]
		}

		osrelease[key] = value
	}

	return osrelease, nil
}
