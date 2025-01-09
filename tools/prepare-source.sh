#!/bin/sh

set -eux

# Since Go 1.21 the version is automatically maintained by the toolchain feature:
go get go@1.22 toolchain@1.22.10

# Update go.mod and go.sum:
go mod tidy
go mod vendor

# Generate all sources (skip vendor/):
go generate ./cmd/... ./internal/... ./pkg/...

# Generate all sources (skip vendor/):
go fmt ./cmd/... ./internal/... ./pkg/...
