#!/usr/bin/bash

# Reusable function, which waits for a given host to respond to SSH
function _instanceWaitSSH() {
  local HOST="$1"

  for LOOP_COUNTER in {0..30}; do
      if ssh-keyscan "$HOST" > /dev/null 2>&1; then
          echo "SSH is up!"
          ssh-keyscan "$HOST" | sudo tee -a /root/.ssh/known_hosts
          break
      fi
      echo "Retrying in 5 seconds... $LOOP_COUNTER"
      sleep 5
  done
}

function _instanceCheck() {
  echo "✔️ Instance checking"
  local _ssh="$1"

  # Check if postgres is installed
  $_ssh rpm -q postgresql dummy

  # Verify subscribe status. Loop check since the system may not be registered such early(RHEL only)
  if [[ "$ID" == "rhel" ]]; then
    set +eu
    for LOOP_COUNTER in {1..10}; do
        subscribe_org_id=$($_ssh sudo subscription-manager identity | grep 'org ID')
        if [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]; then
            echo "System is subscribed."
            break
        else
            echo "System is not subscribed. Retrying in 30 seconds...($LOOP_COUNTER/10)"
            sleep 30
        fi
    done
    set -eu
    [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]

    # Unregister subscription
    $_ssh sudo subscription-manager unregister
  else
    echo "Not RHEL OS. Skip subscription check."
  fi
}

WORKER_REFRESH_TOKEN_PATH="/etc/osbuild-worker/token"

# Fetch a JWT token.
# The token is fetched using the refresh token configured in the worker.
function access_token {
  local refresh_token
  refresh_token="$(cat $WORKER_REFRESH_TOKEN_PATH)"
  access_token_with_org_id "$refresh_token"
}

# Fetch a JWT token.
# The token is fetched using the refresh token provided as an argument.
function access_token_with_org_id {
  local refresh_token="$1"
  curl --request POST \
    --data "grant_type=refresh_token" \
    --data "refresh_token=$refresh_token" \
    --header "Content-Type: application/x-www-form-urlencoded" \
    --silent \
    --show-error \
    --fail \
    localhost:8081/token | jq -r .access_token
}

# Get the compose status using a JWT token.
# The token is fetched using the refresh token configured in the worker.
function compose_status {
  local compose="$1"
  local refresh_token
  refresh_token="$(cat $WORKER_REFRESH_TOKEN_PATH)"
  compose_status_with_org_id "$compose" "$refresh_token"
}

# Get the compose status using a JWT token.
# The token is fetched using the refresh token provided as the second argument.
function compose_status_with_org_id {
  local compose="$1"
  local refresh_token="$2"
  curl \
    --silent \
    --show-error \
    --fail \
    --header "Authorization: Bearer $(access_token_with_org_id "$refresh_token")" \
    "http://localhost:443/api/image-builder-composer/v2/composes/$compose"
}
