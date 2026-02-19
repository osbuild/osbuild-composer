#!/usr/bin/bash
# vim: sw=2:et:

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

  # Retry loop to wait for instance to be ready
  # This is here especially because of gcp test
  RETRIES=10
  for i in $(seq 1 $RETRIES); do
    echo "Attempt $i of $RETRIES: Checking instance status..."
    if eval "$_ssh true"; then
      echo "Instance is up and ready!"
      break
    else
      echo "Instance is still booting or SSH key not propagated, retrying in 30 seconds..."
      sleep 30
    fi
  done

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
    # NOTE: workaround until fact value becomes configurable in image-builder-cli (https://issues.redhat.com/browse/HMS-9819)
    expected_fact_value="cloudapi-v2"
    IMAGE_BUILDER_EXPERIMENTAL="${IMAGE_BUILDER_EXPERIMENTAL:-}"
    if [[ "${IMAGE_BUILDER_EXPERIMENTAL}" != "" ]]; then
        if echo "${IMAGE_BUILDER_EXPERIMENTAL}" | grep -q "image-builder-manifest-generation=1"; then
          expected_fact_value="image-builder-cli"
        fi
    fi
    if ! grep -q "image-builder.osbuild-composer.api-type: ${expected_fact_value}" <<< "$FACTS"; then
        echo "System doesn't contain the expected image-builder.osbuild-composer facts"
        echo "$FACTS" | grep image-builder
        exit 1
    fi

    # NOTE: workaround until https://issues.redhat.com/browse/HMS-9822 is resolved (Set OSCAP RHSM facts automatically)
    if [[ "${IMAGE_BUILDER_EXPERIMENTAL}" == "" ]]; then
        if [ -n "$OPENSCAP_CUSTOMIZATION_BLOCK" ]; then
            if ! grep -q "image-builder.insights.compliance-profile-id: pci-dss" <<< "$FACTS"; then
                echo "System doesn't contain the expected image-builder.insights facts (profile-id)"
                echo "$FACTS"| grep image-builder
                exit 1
            fi
            if ! grep -q "image-builder.insights.compliance-policy-id: 1af6cced-581c-452c-89cd-33b7bddb816a" <<< "$FACTS"; then
                echo "System doesn't contain the expected image-builder.insights facts (policy-id)"
                echo "$FACTS"| grep image-builder
                exit 1
            fi
        fi
    fi

    verify_modules_customization "$_ssh"

    # Unregister subscription
    $_ssh sudo subscription-manager unregister
  else
    echo "Not RHEL OS. Skip subscription check."
    verify_modules_customization "$_ssh"
  fi

  # Verify that directories and files customization worked as expected
  verify_dirs_files_customization "$_ssh"

  verify_repository_customization "$_ssh"
  verify_openscap_customization "$_ssh"
  verify_cacert_customization "$_ssh"

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

# Verify that CA cert file was extracted
function verify_cacert_customization {
  echo "✔️ Checking CA cert extration"
  local _ssh="$1"
  local _serial="27894af897dd2423607045716438a725f28a6d0b"
  local _cn="Test CA for osbuild"

  if ! $_ssh "test -e /etc/pki/ca-trust/source/anchors/${_serial}.pem"; then
    echo "Anchor CA file does not exist, directory contents:"
    $_ssh "find /etc/pki/ca-trust/source/anchors"
    exit 1
  fi

  if ! $_ssh "grep -q \"${_cn}\" /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"; then
    echo "Extracted CA file is not present, bundle contents:"
    $_ssh "grep '^#' /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
    exit 1
  fi
}


function verify_modules_customization {
  local _ssh="$1"
  if jq -e .customizations.enabled_modules; then
    # NOTE: assumes only one module
    echo "✔️ Checking CA cert extration"
    module_name=$(cat "$REQUEST_FILE" | jq -r .customizations.enabled_modules[0].name)
    module_stream=$(cat "$REQUEST_FILE" | jq -r .customizations.enabled_modules[0].stream)
    echo "Checking if ${module_name} ${module_stream} is installed"
    $_ssh rpm -q "${module_name}" | grep "${module_name}-${module_stream}"

    # Also verify that the module is enabled
    # NOTE: on RHEL, listing enabled modules requires the machine be subscribed,
    # which we always do in these tests
    # The -y option is needed to accept gpg key import for Google Compute
    # Engine repositories.
    if ! check_modules=$($_ssh sudo dnf -y module list --enabled); then
      echo "Error getting module list"
      echo "${check_modules}"
      exit 1
    fi
    pattern="${module_name}\s+${module_stream}"
    if grep -P "${pattern}" <<< "${check_modules}"; then
      echo "module ${module_name}:${module_stream} is enabled"
    else
      echo "module ${module_name}:${module_stream} is not enabled"
      echo 'Output from "dnf module list --enabled":'
      echo "${check_modules}"
      exit 1
    fi
  fi
}
