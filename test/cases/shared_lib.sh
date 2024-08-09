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
    local key="$1"
    local fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
        if nvrGreaterOrEqual "weldr-client" "35.6" 2> /dev/null; then
            key=".[0]${key}"
        fi
    fi
    jq -r "${key}" "${fname}"
}

# Colorful timestamped output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

function yellowprint {
    echo -e "\033[1;33m[$(date -Isecond)] ${1}\033[0m"
}

function redprint {
    echo -e "\033[1;31m[$(date -Isecond)] ${1}\033[0m"
}
