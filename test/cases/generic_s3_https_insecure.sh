#!/bin/bash
set -euo pipefail

# Test upload to HTTPS S3 server without verifying the SSL certificate
/usr/libexec/osbuild-composer-test/generic_s3_https_test.sh "no"
