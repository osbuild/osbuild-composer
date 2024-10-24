#!/usr/bin/env bash

./tools/dbtest-run-migrations.sh

go version
go test -tags=integration ./cmd/osbuild-composer-dbjobqueue-tests
go test -tags=integration ./cmd/osbuild-service-maintenance
