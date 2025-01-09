#!/bin/sh

set -eux

# Pin Go and toolbox versions at a reasonable version
go get go@1.22.0 toolchain@1.22.0

# Update go.mod and go.sum:
go mod tidy
go mod vendor

# Generate all sources (skip vendor/):
go generate ./cmd/... ./internal/... ./pkg/...

# Generate all sources (skip vendor/):
go fmt ./cmd/... ./internal/... ./pkg/...
