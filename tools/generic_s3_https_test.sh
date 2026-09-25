#!/bin/bash
set -euo pipefail

SECURE=${1}

if [[ ${SECURE} == "yes" ]]; then
    CA_CERT_NAME="public.crt"
fi

CERTGEN_VERSION="v1.2.0"

TEMPDIR=$(mktemp -d)

CERTS_DIR=/var/lib/s3-certs
sudo rm -rf "${CERTS_DIR}" || true
sudo mkdir "${CERTS_DIR}"

function cleanup() {
    sudo rm -rf "$TEMPDIR" || true
    sudo rm -rf "$CERTS_DIR" || true
}
trap cleanup EXIT

pushd "${TEMPDIR}"
curl -L -o certgen "https://github.com/minio/certgen/releases/download/${CERTGEN_VERSION}/certgen-linux-amd64"
chmod +x certgen
./certgen -host localhost
sudo mv private.key public.crt "${CERTS_DIR}"
# RustFS loads TLS only from RUSTFS_TLS_PATH, and only when the files are named
# rustfs_cert.pem / rustfs_key.pem. Keep public.crt for the AWS CA bundle.
sudo cp "${CERTS_DIR}/public.crt" "${CERTS_DIR}/rustfs_cert.pem"
sudo cp "${CERTS_DIR}/private.key" "${CERTS_DIR}/rustfs_key.pem"
# The container runs as the rustfs user (UID 10001).
sudo chown -R 10001:10001 "${CERTS_DIR}"
popd

# Test upload to HTTPS S3 server
/usr/libexec/osbuild-composer-test/generic_s3_test.sh "${CERTS_DIR}" "${CA_CERT_NAME:-""}"
