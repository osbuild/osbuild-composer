package common

import "runtime"

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
