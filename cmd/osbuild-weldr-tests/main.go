// osbuild-tests runs all of the osbuild integration tests against a live server
// Copyright (C) 2020 by Red Hat, Inc.
package main

import (
	"github.com/osbuild/osbuild-composer/internal/weldrcheck"
)

func main() {
	weldrcheck.Run("/run/weldr/api.socket")
}
