//go:build tools

// This file is here to just explicitly tell `go mod vendor` that we depend
// on oapi-codegen. Without this file, `go generate ./...` in Go >= 1.14 gets
// confused because oapi-codegen is not being vendored.
//
// This is apparently the conventional way, see:
// https://stackoverflow.com/questions/52428230/how-do-go-modules-work-with-installable-commands
// https://github.com/golang/go/issues/29516

package main

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
