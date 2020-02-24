package common

import "runtime"

func CurrentArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	} else if runtime.GOARCH == "arm64" {
		return "aarch64"
	} else if runtime.GOARCH == "ppc64le" {
		return "ppc64le"
	} else if runtime.GOARCH == "s390x" {
		return "s390x"
	} else {
		panic("unsupported architecture")
	}
}
