#!/usr/bin/env bash

WORKDIR=$(readlink -f pkg/jobqueue/dbjobqueue/schemas)
$(go env GOPATH)/bin/tern migrate -m "$WORKDIR"
