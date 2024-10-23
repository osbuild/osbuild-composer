#!/usr/bin/env bash

go version
go test -tags=integration ./cmd/osbuild-composer-dbjobqueue-tests
go test -tags=integration ./cmd/osbuild-service-maintenance
