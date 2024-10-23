#!/usr/bin/env bash

pushd $(mktemp -d)
go mod init temp
go install github.com/jackc/tern@latest
popd
