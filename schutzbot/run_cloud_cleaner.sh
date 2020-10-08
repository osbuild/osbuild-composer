#!/bin/bash
set -euox pipefail

CLEANER_CMD=/usr/libexec/osbuild-composer/cloud-cleaner

env | sort
$CLEANER_CMD
