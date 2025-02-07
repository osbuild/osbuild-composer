#!/bin/sh

set -eu

GO_MINOR_VERSION="1.22"
GO_VERSION="${GO_MINOR_VERSION}.6"

# Check latest Go version for the minor we're using
LATEST=$(curl -s https://endoflife.date/api/go/"${GO_MINOR_VERSION}".json  | jq -r .latest)
if test "$LATEST" != "$GO_VERSION"; then
    echo "NOTE: A new minor release is available (${LATEST}), consider bumping the project version (${GO_VERSION})"
fi

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
