#!/bin/sh

set -eux

GO_VERSION=1.13.15
GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION

# this is the official way to get a different version of golang
# see https://golang.org/doc/install#extra_versions
go get golang.org/dl/go$GO_VERSION
$GO_BINARY download

# ensure that the code is formatted correctly, ...
$GO_BINARY fmt ./...

# ... go.mod and go.sum are up to date, ...
$GO_BINARY mod tidy
$GO_BINARY mod vendor

# ... and all code has been regenerated from its sources.
$GO_BINARY generate ./...
