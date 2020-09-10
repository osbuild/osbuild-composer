#!/bin/bash
set -euox pipefail

CLEANER_CMD=/usr/libexec/tests/osbuild-composer/cloud-cleaner

$CLEANER_CMD
