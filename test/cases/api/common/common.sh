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

    FACTS=$($_ssh sudo subscription-manager facts)
    if ! grep -q "image-builder.osbuild-composer.api-type: cloudapi-v2" <<< "$FACTS"; then
        echo "System doesn't contain the expected osbuild facts"
        echo "$FACTS" | grep image-builder
        exit 1
    fi

    # Unregister subscription
    $_ssh sudo subscription-manager unregister
  else
    echo "Not RHEL OS. Skip subscription check."
  fi

  # Verify that directories and files customization worked as expected
  verify_dirs_files_customization "$_ssh"

  verify_repository_customization "$_ssh"
# TODO: Remove once Openscap works on el-10
  if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
    yellowprint "OpenSCAP not supported on ${ID}-${VERSION_ID} now. No verification made!"
  else
    verify_openscap_customization "$_ssh"
  fi

  echo "✔️ Checking timezone customization"
  TZ=$($_ssh timedatectl show  -p Timezone --value)
  if [ "$TZ" != "Europe/Prague" ]; then
      echo "Timezone $TZ isn't Europe/Prague"
      exit 1
  fi

  echo "✔️ Checking firewall customization"
  if $_ssh rpm -q firewalld; then
      FW_SERVICES=$($_ssh sudo firewall-cmd --list-services)
      if ! grep -q "nfs" <<< "$FW_SERVICES"; then
          echo "firewalld nfs service isn't enabled: $FW_SERVICES"
          exit 1
      fi
  else
      echo "firewalld not available on host, that's fine"
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

# Verify that directories and files customization worked as expected
function verify_dirs_files_customization {
  echo "✔️ Checking custom directories and files"
  local _ssh="$1"
  local _error=0

  # verify that `/etc/custom_dir/dir1` exists and has mode `0775`
  local cust_dir1_mode
  cust_dir1_mode=$($_ssh stat -c '%a' /etc/custom_dir/dir1)
  if [[ "$cust_dir1_mode" != "775" ]]; then
    echo "Directory /etc/custom_dir/dir1 has wrong mode: $cust_dir1_mode"
    _error=1
  fi

  # verify that `/etc/custom_dir/custom_file.txt` exists and contains `image builder is the best\n`
  local cust_file_content
  cust_file_content=$($_ssh cat /etc/custom_dir/custom_file.txt)
  if [[ "$cust_file_content" != "image builder is the best" ]]; then
    echo "File /etc/custom_dir/custom_file.txt has wrong content: $cust_file_content"
    _error=1
  fi

  # verify that `/etc/custom_dir2/empty_file.txt` exists and is empty
  local cust_file2_content
  cust_file2_content=$($_ssh cat /etc/custom_dir2/empty_file.txt)
  if [[ "$cust_file2_content" != "" ]]; then
    echo "File /etc/custom_dir2/empty_file.txt has wrong content: $cust_file2_content"
    _error=1
  fi

  if [[ "$_error" == "1" ]]; then
    echo "Testing of custom directories and files failed."
    exit 1
  fi
}

# Verify that repository customizations worked as expected
function verify_repository_customization {
  echo "✔️ Checking custom repositories"
  local _ssh="$1"
  local _error=0

  local _custom_repo_file="/etc/yum.repos.d/example.repo"
  local _key_file_path="/etc/pki/rpm-gpg/RPM-GPG-KEY-example-0"

  # verify that `/etc/yum.repos.d/example.repo` exists
  # and contains path to gpg key file
  local cust_repo_contains_key_path
  cust_repo_contains_key_path=$($_ssh cat "$_custom_repo_file" | grep -c "${_key_file_path}")
  if [[ "$cust_repo_contains_key_path" -le 0 ]]; then
    echo "File $_custom_repo_file does not contain ${_key_file_path}}"
    _error=1
  fi

  # verify that gpg key file has been saved to image
  # and the contents match the expected gpg key
  local local_key remote_key key_diff
  local_key=$(echo -e "$CUSTOM_GPG_KEY")
  remote_key=$($_ssh cat "${_key_file_path}")
  key_diff=$(diff <(echo "$local_key") <(echo "$remote_key") | wc -l)
  if [[ "$key_diff" -gt 0 ]]; then
    echo "File $_key_file_path has wrong content"
    _error=1
  fi

  if [[ "$_error" == "1" ]]; then
    echo "Testing of custom repositories failed."
    exit 1
  fi
}

# Verify that tailoring file was created
function verify_openscap_customization {
  echo "✔️ Checking OpenSCAP customizations"
  local _ssh="$1"
  local _error=0

  # NOTE: We are only checking the creation of the tailoring file and ensuring it exists
  # since running openscap tests here requires more memory and causes some out-of-memory issues.
  local tailoring_file_content
  tailoring_file_path="/oscap_data/tailoring.xml"
  tailoring_file_content=$($_ssh cat "${tailoring_file_path}" \
      | grep 'idref="xccdf_org.ssgproject.content_rule_rpm_verify_permissions" selected="false"' -c
  )
  if [[ "$tailoring_file_content" -eq 0 ]]; then
    echo "File ${tailoring_file_path} has wrong content"
    _error=1
  fi

  if [[ "$_error" == "1" ]]; then
    echo "Testing of OpenSCAP customizations has failed."
    exit 1
  fi
}
