#!/usr/bin/bash

function nvrGreaterOrEqual {
    local rpm_name=$1
    local min_version=$2

    set +e

    rpm_version=$(rpm -q --qf "%{version}" "${rpm_name}")
    rpmdev-vercmp "${rpm_version}" "${min_version}"
    if [ "$?" != "12" ]; then
        # 0 - rpm_version == min_version
        # 11 - rpm_version > min_version
        # 12 - rpm_version < min_version
        echo "DEBUG: ${rpm_version} >= ${min_version}"
        set -e
        return
    fi

    set -e
    false
}
