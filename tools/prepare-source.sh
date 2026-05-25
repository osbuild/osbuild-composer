#!/bin/sh

set -eu

# 1.24.12
GO_MINOR_VERSION="1.24"
GO_VERSION="${GO_MINOR_VERSION}.12"

set -x

# Pin Go and toolbox versions
go get "go@${GO_VERSION}" "toolchain@${GO_VERSION}"

# Update go.mod and go.sum:
go mod tidy
go mod vendor

# Generate all sources (skip vendor/):
go generate ./cmd/... ./internal/... ./pkg/...

# Format all sources (skip vendor/):
go fmt ./cmd/... ./internal/... ./pkg/...
