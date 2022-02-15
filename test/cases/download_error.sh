#!/usr/bin/bash

set -euxo pipefail

# Colorful timestamped output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

ARTIFACTS=ci-artifacts
mkdir -p "${ARTIFACTS}"

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

#
# Provision the software under test.
#

/usr/libexec/osbuild-composer-test/provision.sh

#
# Set up the database queue
#
if which podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

# Start the db
sudo ${CONTAINER_RUNTIME} run -d --name osbuild-composer-db \
    --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
    --health-timeout 2s --health-retries 10 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=osbuildcomposer \
    -p 5432:5432 \
    quay.io/osbuild/postgres:13-alpine

# Dump the logs once to have a little more output
sudo ${CONTAINER_RUNTIME} logs osbuild-composer-db

# Initialize a module in a temp dir so we can get tern without introducing
# vendoring inconsistency
pushd "$(mktemp -d)"
sudo dnf install -y go
go mod init temp
go get github.com/jackc/tern
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
      go run github.com/jackc/tern migrate -m /usr/share/tests/osbuild-composer/schemas
popd

cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
log_level = "debug"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
[koji.aws_config]
bucket = "${AWS_BUCKET}"
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


WORKDIR=$(mktemp -d)
KILL_PIDS=()
function cleanup() {
  set +eu
  sudo rm -rf "$WORKDIR"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanup EXIT

# make a dummy rpm and repo to test payload_repositories
sudo dnf install -y rpm-build createrepo
DUMMYRPMDIR=$(mktemp -d)
DUMMYSPECFILE="$DUMMYRPMDIR/dummy.spec"
PAYLOAD_REPO_PORT="9999"

pushd "$DUMMYRPMDIR"

cat <<EOF > "$DUMMYSPECFILE"
#----------- spec file starts ---------------
Name:                   dummy
Version:                1.0.0
Release:                0
BuildArch:              noarch
Vendor:                 dummy
Summary:                Provides %{name}
License:                BSD
Provides:               dummy

%description
%{summary}

%files
EOF

mkdir -p "DUMMYRPMDIR/rpmbuild"
rpmbuild --quiet --define "_topdir $DUMMYRPMDIR/rpmbuild" -bb "$DUMMYSPECFILE"

mkdir -p "$DUMMYRPMDIR/repo"
cp "$DUMMYRPMDIR"/rpmbuild/RPMS/noarch/*rpm "$DUMMYRPMDIR/repo"
pushd "$DUMMYRPMDIR/repo"
createrepo .
sudo python3 -m http.server "$PAYLOAD_REPO_PORT" &
KILL_PIDS+=("$!")
popd
popd


REQUEST_FILE="${WORKDIR}/request.json"
cat > "$REQUEST_FILE" << EOF
{
    "distribution": "centos-8",
    "image_request": {
        "architecture": "x86_64",
        "image_type": "aws",
        "repositories": [
            {
            "baseurl": "http://mirror.centos.org/centos/8-stream/extras/x86_64/os/",
            "rhsm": false
            },
            {
                "baseurl": "http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/",
                "rhsm": false
            },
            {
            "baseurl": "http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/",
            "rhsm": false
            }
        ],
        "upload_options": {
            "region": "trucmuche"
        }
    }
}
EOF

#
# Send the request and wait for the job to finish.
#
# Separate `curl` and `jq` commands here, because piping them together hides
# the server's response in case of an error.
#

function collectMetrics(){
    METRICS_OUTPUT=$(curl \
                          --cacert /etc/osbuild-composer/ca-crt.pem \
                          --key /etc/osbuild-composer/client-key.pem \
                          --cert /etc/osbuild-composer/client-crt.pem \
                          https://localhost/metrics)

    echo "$METRICS_OUTPUT" | grep "^image_builder_composer_total_compose_requests" | cut -f2 -d' '
}

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
                 https://localhost/api/image-builder-composer/v3/compose)

    test "$HTTPSTATUS" = "201"
    COMPOSE_ID=$(jq -r '.id' "$OUTPUT")
}

function waitForState() {
    local DESIRED_STATE="${1:-success}"

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
                exit 1
                ;;
            *)
                echo "API returned unexpected image_status.status value: '$COMPOSE_STATUS'"
                exit 1
                ;;
        esac

        sleep 30
    done
}

# break dnf-json, make it return always the same false url
sudo systemctl disable --now osbuild-composer.service osbuild-composer-api.socket osbuild-worker@1.service osbuild-dnf-json.socket dnf-json.service
sudo sed -i '/package.remote_location/c\                "remote_location": "http://pouet.pouet.com",' /usr/libexec/osbuild-composer/dnf-json
sudo systemctl enable --now osbuild-composer-api.socket osbuild-dnf-json.socket osbuild-worker@1.service

# downloading fails
sendCompose "$REQUEST_FILE"
waitForState "failure"

journalctl -u osbuild-worker@1.service  | grep pouet.com
