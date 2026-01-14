#!/usr/bin/bash

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

/usr/libexec/osbuild-composer-test/provision.sh

# Check available container runtime
if type -p podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

function dump_db() {
  # Save the result, including the manifest, for the job, straight from the db
  sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT type, args, result FROM jobs" \
    | sudo tee "${ARTIFACTS}/build-result.txt" > /dev/null
}

KILL_PIDS=()
function cleanups() {
  greenprint "Cleaning up"
  set +eu

  # save manifest
  dump_db

  sudo "${CONTAINER_RUNTIME}" kill "${DB_CONTAINER_NAME}"
  sudo "${CONTAINER_RUNTIME}" rm "${DB_CONTAINER_NAME}"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanups EXIT

# Start the db
DB_CONTAINER_NAME="osbuild-composer-db"
sudo "${CONTAINER_RUNTIME}" run -d --name "${DB_CONTAINER_NAME}" \
    --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
    --health-timeout 2s --health-retries 10 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=osbuildcomposer \
    -p 5432:5432 \
    --net host \
    quay.io/osbuild/postgres:13-alpine

sudo "${CONTAINER_RUNTIME}" logs osbuild-composer-db

pushd "$(mktemp -d)"
sudo dnf install -y go
go mod init temp
go install github.com/jackc/tern@latest
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
    "$(go env GOPATH)"/bin/tern migrate -m /usr/share/tests/osbuild-composer/schemas
popd

cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
ignore_missing_repos = true
log_level = "debug"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
[worker]
allowed_domains = [ "localhost", "worker.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
pg_host = "localhost"
pg_port = "5432"
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
pg_max_conns = 10
EOF

sudo systemctl restart osbuild-composer

curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/image-builder-composer/v2/openapi | jq .


function sendCompose() {
    OUTPUT=$(mktemp)
    HTTPSTATUS=$(curl \
                 --silent \
                 --show-error \
                 --cacert /etc/osbuild-composer/ca-crt.pem \
                 --key /etc/osbuild-composer/client-key.pem \
                 --cert /etc/osbuild-composer/client-crt.pem \
                 --header 'Content-Type: application/json' \
                 --request POST \
                 --data @"$1" \
                 --write-out '%{http_code}' \
                 --output "$OUTPUT" \
                 https://localhost/api/image-builder-composer/v2/compose)

    if [ "$HTTPSTATUS" != "201" ]; then
        redprint "Sending compose request failed:"
        cat "$OUTPUT"
    fi

    test "$HTTPSTATUS" = "201"

    COMPOSE_ID=$(jq -r '.id' "$OUTPUT")
}


function waitForState() {
    local DESIRED_STATE="success"

    while true
    do
        OUTPUT=$(curl \
                     --silent \
                     --show-error \
                     --cacert /etc/osbuild-composer/ca-crt.pem \
                     --key /etc/osbuild-composer/client-key.pem \
                     --cert /etc/osbuild-composer/client-crt.pem \
                     "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID")

        COMPOSE_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.status')
        UPLOAD_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.status')
        UPLOAD_OPTIONS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.options')

        case "$COMPOSE_STATUS" in
            "$DESIRED_STATE")
                break
                ;;
            # all valid status values for a compose which hasn't finished yet
            "pending"|"building"|"uploading"|"registering")
                ;;
            # default undesired state
            "failure")
                echo "Image compose failed"
                echo "API output: $OUTPUT"
                exit 1
                ;;
            *)
                echo "API returned unexpected image_status.status value: '$COMPOSE_STATUS'"
                echo "API output: $OUTPUT"
                exit 1
                ;;
        esac

        sleep 30
    done

    # export for use in subcases
    export UPLOAD_OPTIONS
}

WORKDIR=$(mktemp -d)
REQ="${WORKDIR}/compose_request.json"
ARCH=$(uname -m)

cat > "$REQ" << EOF
{
  "bootc": {
    "reference": "quay.io/centos-bootc/centos-bootc:stream9"
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "guest-image",
    "repositories": [],
    "upload_options": {}
  }
}
EOF

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
KILL_PIDS+=("$!")

sendCompose "$REQ"
waitForState
echo "compose status:"
curl \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID"
test "$UPLOAD_STATUS" = "success"
