#!/usr/bin/bash

function nvrGreaterOrEqual {
    local rpm_name=$1
    local min_version=$2

    set +e

    rpm_version=$(rpm -q --qf "%{version}" "${rpm_name}")
    rpmdev-vercmp "${rpm_version}" "${min_version}" 1>&2
    if [ "$?" != "12" ]; then
        # 0 - rpm_version == min_version
        # 11 - rpm_version > min_version
        # 12 - rpm_version < min_version
        echo "DEBUG: ${rpm_version} >= ${min_version}" 1>&2
        set -e
        return
    fi

    set -e
    false
}

function get_build_info() {
    local filter="$1"
    local fname="$2"
    if rpm -q --quiet weldr-client; then
        filter=".body${filter}"  # oldest structure, pre v35.6
        if nvrGreaterOrEqual "weldr-client" "35.6" 2> /dev/null; then
            filter=".[0]${filter}"  # changed to array in v35.6
        fi
    fi
    jq -r "${filter}" "${fname}"
}

# Returns the compose ID given the path to a file that contains the response
# from a "compose start" call.
function get_compose_id() {
    local response_file="${1}"
    get_build_info ".build_id" "${response_file}"
}

# Return the status of a compose given a compose ID.
# This function handles response structure differences in various weldr-client
# / composer-cli versions. The status string also differs for newer (>= v36.0)
# of the client.
function get_compose_status() {
    local compose_id="$1"

    local filter=".body.queue_status"  # oldest structure, pre v35.6
    if nvrGreaterOrEqual "weldr-client" "35.6" 2> /dev/null; then
        filter=".[0].body.queue_status"  # changed to array in v35.6
    fi

    if nvrGreaterOrEqual "weldr-client" "36.0" 2> /dev/null; then
        filter='.[].body | select(.kind == "ComposeStatus") | .status'  # changed using cloud API since v36.0
    fi

    sudo composer-cli --json compose info "${compose_id}" | jq -r "${filter}"
}

# Function to wait for a compose to finish
function wait_for_compose() {
    local compose_id="$1"
    local timeout=${2:-1200}
    local compose_status

    if [[ -z "$compose_id" ]]; then
        redprint "ERROR (wait_for_compose): No compose ID provided"
        exit 1
    fi

    greenprint "⏱ Waiting for compose to finish: ${compose_id}"
    while [[ $timeout -gt 0 ]]; do
        compose_status=$(get_compose_status "$compose_id")

        # Is the compose finished?
        if [[ $compose_status != "RUNNING" ]] && \
            [[ $compose_status != "WAITING" ]] && \
            [[ $compose_status != "pending" ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 30
        timeout=$((timeout - 30))
    done

    # Get the last compose status in case the loop above did not break but
    # timed out
    compose_status=$(get_compose_status "$compose_id")

    if [[ $compose_status == "RUNNING" || $compose_status == "WAITING" || $compose_status == "pending" ]] && [[ $timeout -le 0 ]]; then
        redprint "ERROR: Compose did not finish in time"
        exit 1
    fi

    greenprint "INFO: Compose finished with status: ${compose_status}"

    # Return the status of the compose
    echo "$compose_status"
}

# Colorful timestamped output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] $*\033[0m" >&2
}

function yellowprint {
    echo -e "\033[1;33m[$(date -Isecond)] $*\033[0m" >&2
}

function redprint {
    echo -e "\033[1;31m[$(date -Isecond)] $*\033[0m" >&2
}

# Helper for GitLab foldable sections
function section_start() {
    local section_id="${1}_$$"
    local section_title=$2
    local collapsed=${3:-true}
    local params=""
    [ "$collapsed" = "true" ] && params="[collapsed=true]"

    printf "\e[0Ksection_start:%s:%s%s\r\e[0K\e[1;36m%s\e[0m\n" "$(date +%s)" "$section_id" "$params" "$section_title"
}

# Helper for GitLab foldable sections
function section_end() {
    local section_id="${1}_$$"
    printf "\e[0Ksection_end:%s:%s\r\e[0K\n" "$(date +%s)" "$section_id"
}
