#!/usr/bin/bash

#
# Combined unit tests for the Cloud API v2 that do NOT depend on a specific
# image type or cloud provider. Covers:
#   1. Compose failure path (non-existent package -> "failure" status)
#   2. Worker crash / job retry (DB retries column)
#   3. OAuth2/JWT authentication (mock OpenID provider, Bearer tokens)
#
# These tests were extracted from api.sh because they exercise infrastructure
# logic that is orthogonal to the image-type matrix, eliminating ~50 redundant
# runs per pipeline.
#

set -uo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# ---------------------------------------------------------------------------
# Test result tracking
# ---------------------------------------------------------------------------
declare -a TEST_NAMES=()
declare -a TEST_RESULTS=()

function record_result() {
    TEST_NAMES+=("$1")
    TEST_RESULTS+=("$2")
}

function print_summary() {
    echo ""
    echo "======================================================================"
    echo "  TEST SUMMARY"
    echo "======================================================================"
    local failed=0
    for i in "${!TEST_NAMES[@]}"; do
        local status="${TEST_RESULTS[$i]}"
        local name="${TEST_NAMES[$i]}"
        if [ "$status" = "PASS" ]; then
            printf "  %-50s [PASS]\n" "$name"
        elif [ "$status" = "SKIP" ]; then
            printf "  %-50s [SKIP]\n" "$name"
        else
            printf "  %-50s [FAIL]\n" "$name"
            failed=1
        fi
    done
    echo "======================================================================"
    if [ "$failed" = 1 ]; then
        echo "  RESULT: SOME TESTS FAILED"
    else
        echo "  RESULT: ALL TESTS PASSED"
    fi
    echo "======================================================================"
    echo ""
    return $failed
}

# ---------------------------------------------------------------------------
# Provision and setup (failures here are fatal -- no point running tests)
# ---------------------------------------------------------------------------
set -e

/usr/libexec/osbuild-composer-test/provision.sh

source /usr/libexec/tests/osbuild-composer/api/common/composer-db.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh

greenprint "Setting up PostgreSQL database"
setup_db

greenprint "Writing TLS composer config"
write_tls_composer_config

sudo systemctl restart osbuild-composer

WORKDIR=$(mktemp -d)
function cleanups() {
    set +eu
    print_summary
    FINAL_RC=$?

    greenprint "Cleaning up"
    /usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh stop || true
    teardown_db
    sudo rm -rf "$WORKDIR"

    exit $FINAL_RC
}
trap cleanups EXIT

ARCH=$(uname -m)
DISTRO="$ID-${VERSION_ID}"
REQUEST_FILE="${WORKDIR}/compose_request.json"

cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "bash"
    ]
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "guest-image",
    "repositories": $(jq "[.\"$ARCH\"[] | select(.baseurl | test(\"rhvpn\") | not)]" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_targets": [
      {
        "type": "aws.s3",
        "upload_options": {
          "region": "us-east-1"
        }
      }
    ]
  }
}
EOF

set +e

# ============================================================================
# Section 1: Compose Failure Path
# ============================================================================
greenprint "====== Section 1: Compose Failure Path ======"

REQUEST_FILE_FAIL="${WORKDIR}/request_fail.json"
jq '.customizations.packages = [ "jesuisunpaquetquinexistepas" ]' "$REQUEST_FILE" > "$REQUEST_FILE_FAIL"

greenprint "Sending compose with non-existent package (expect failure)"
if sendCompose "$REQUEST_FILE_FAIL" && waitForState "failure"; then
    record_result "Compose failure path (bad package)" "PASS"
else
    record_result "Compose failure path (bad package)" "FAIL"
fi

# ============================================================================
# Section 2: Job Retry
# ============================================================================
greenprint "====== Section 2: Job Retry ======"

RETRY_PASSED=true

# Clear worker's rpmmd cache so the depsolve job has to re-download repo
# metadata from scratch (~5-10 s), giving us a reliable window to kill the
# worker while a job is in-flight.
sudo rm -rf /var/cache/osbuild-worker/rpmmd

greenprint "Sending compose for retry test"
if ! sendCompose "$REQUEST_FILE"; then
    RETRY_PASSED=false
else
    # Poll the DB (not the API) for any job the worker has dequeued but not
    # yet finished.  With a cold rpmmd cache the depsolve job stays in this
    # state for several seconds — long enough for the 0.5 s poll to catch it.
    ORPHANED_JOB=""
    for _i in {1..60}; do
        ORPHANED_JOB=$(sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" \
            psql -t -A -U postgres -d osbuildcomposer -c \
            "SELECT id FROM jobs WHERE started_at IS NOT NULL AND finished_at IS NULL LIMIT 1" 2>/dev/null | tr -d '[:space:]')
        if [ -n "$ORPHANED_JOB" ]; then
            break
        fi
        sleep 0.5
    done

    if [ -z "$ORPHANED_JOB" ]; then
        echo "Failed to catch any job in running state"
        RETRY_PASSED=false
    else
        greenprint "Caught running job $ORPHANED_JOB, killing worker to simulate crash"
        sudo systemctl kill -s SIGKILL "osbuild-remote-worker@*"

        RETRIED=0
        for RETRY in {1..10}; do
            ROWS=$(sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" \
                psql -U postgres -d osbuildcomposer -c \
                "SELECT retries FROM jobs WHERE id = '$ORPHANED_JOB' AND retries >= 1")
            if grep -q "1 row" <<< "$ROWS"; then
                RETRIED=1
                break
            else
                echo "Waiting until job is retried ($RETRY/10)"
                sleep 30
            fi
        done

        if [ "$RETRIED" != 1 ]; then
            echo "Job $ORPHANED_JOB wasn't retried after killing the worker"
            RETRY_PASSED=false
        else
            greenprint "Job was retried (retries>=1 in DB). Cleaning up."
            sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" \
                psql -U postgres -d osbuildcomposer -c \
                "DELETE FROM job_dependencies WHERE job_id = '$COMPOSE_ID' OR dependency_id = '$COMPOSE_ID';
                 DELETE FROM heartbeats WHERE id IN (SELECT id FROM jobs WHERE id = '$COMPOSE_ID' OR id = '$ORPHANED_JOB');
                 DELETE FROM jobs WHERE id = '$COMPOSE_ID' OR id = '$ORPHANED_JOB';"
        fi
        sudo systemctl restart "osbuild-remote-worker@localhost:8700.service"
    fi
fi

if [ "$RETRY_PASSED" = true ]; then
    record_result "Job retry (worker crash)" "PASS"
else
    record_result "Job retry (worker crash)" "FAIL"
fi

# ============================================================================
# Section 3: OAuth2/JWT
# ============================================================================
greenprint "====== Section 3: OAuth2/JWT ======"

OAUTH_PASSED=true

greenprint "Reconfiguring composer for JWT authentication"
write_jwt_composer_config

REFRESH_TOKEN="offlineToken"
sudo tee /etc/osbuild-worker/token > /dev/null <<< "$REFRESH_TOKEN"

sudo cp -a /usr/share/tests/osbuild-composer/worker/osbuild-worker-jwt.toml \
    /etc/osbuild-worker/osbuild-worker.toml

greenprint "Starting mock OpenID providers"
/usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh start

sudo systemctl restart osbuild-composer

greenprint "Waiting for token endpoint to become available"
WAIT_COUNT=0
until curl --data "grant_type=refresh_token" --output /dev/null --silent --fail localhost:8081/token; do
    sleep 0.5
    WAIT_COUNT=$((WAIT_COUNT + 1))
    if [ "$WAIT_COUNT" -gt 60 ]; then
        echo "Token endpoint did not become available within 30s"
        OAUTH_PASSED=false
        break
    fi
done

if [ "$OAUTH_PASSED" = true ]; then
    TOKEN="$(curl --request POST \
            --data "grant_type=refresh_token" \
            --data "refresh_token=$REFRESH_TOKEN" \
            --header "Content-Type: application/x-www-form-urlencoded" \
            --silent \
            --show-error \
            --fail \
            localhost:8081/token | jq -r .access_token)"

    greenprint "Testing: GET /openapi with valid Bearer token -> 200"
    RESP="$(curl --silent --output /dev/null --write-out '%{http_code}' \
            --header "Authorization: Bearer $TOKEN" \
            http://localhost:443/api/image-builder-composer/v2/openapi)"
    if [ "$RESP" != "200" ]; then
        echo "Expected 200, got $RESP for /openapi with valid token"
        OAUTH_PASSED=false
    fi

    greenprint "Testing: GET /openapi with bad Bearer token -> 200 (public endpoint)"
    RESP="$(curl --silent --output /dev/null --write-out '%{http_code}' \
            --header "Authorization: Bearer badtoken" \
            http://localhost:443/api/image-builder-composer/v2/openapi)"
    if [ "$RESP" != "200" ]; then
        echo "Expected 200, got $RESP for /openapi with bad token"
        OAUTH_PASSED=false
    fi

    DUMMY_UUID="00000000-0000-0000-0000-000000000000"
    greenprint "Testing: GET /composes/{id} with bad Bearer token -> 401"
    RESP="$(curl --silent --output /dev/null --write-out '%{http_code}' \
            --header "Authorization: Bearer badtoken" \
            http://localhost:443/api/image-builder-composer/v2/composes/"$DUMMY_UUID")"
    if [ "$RESP" != "401" ]; then
        echo "Expected 401, got $RESP for /composes with bad token"
        OAUTH_PASSED=false
    fi

    greenprint "Restarting worker with OAuth config"
    sudo systemctl restart osbuild-remote-worker@localhost:8700.service
    if ! sudo systemctl is-active --quiet osbuild-remote-worker@localhost:8700.service; then
        echo "Worker failed to restart with OAuth config"
        OAUTH_PASSED=false
    fi
fi

if [ "$OAUTH_PASSED" = true ]; then
    record_result "OAuth2/JWT authentication" "PASS"
else
    record_result "OAuth2/JWT authentication" "FAIL"
fi

# The EXIT handler calls print_summary and exits with the appropriate code
