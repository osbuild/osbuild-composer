#!/bin/sh

set -eux

GO_VERSION=1.12.17
GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION

# this is the official way to get a different version of golang
# see https://golang.org/doc/install#extra_versions
go get golang.org/dl/go$GO_VERSION
$GO_BINARY download

# prepare the sources
$GO_BINARY fmt ./...
$GO_BINARY mod tidy
$GO_BINARY mod vendor
