#!/bin/sh

set -eux

go fmt ./...
go mod tidy
go mod vendor
