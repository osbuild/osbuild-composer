#!/bin/bash
set -euo pipefail

# Test upload to HTTP S3 server
/usr/libexec/osbuild-composer-test/generic_s3_test.sh
