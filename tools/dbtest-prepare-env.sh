#!/usr/bin/env bash

pushd "$(mktemp -d)" || exit 1
go mod init temp
go install github.com/jackc/tern@latest
popd || exit
