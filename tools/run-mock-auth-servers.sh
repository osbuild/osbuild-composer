#!/bin/bash
set -eu

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

servers_start() {
    greenprint "Starting mock JWT AUTH servers"
    # Spin up an https instance for the composer-api and worker-api; the auth handler needs to hit an ssl `/certs` endpoint
    sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -a localhost:8082 -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem -cert /etc/osbuild-composer/composer-crt.pem -key /etc/osbuild-composer/composer-key.pem &
    # Spin up an http instance for the worker client to bypass the need to specify an extra CA
    sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -a localhost:8081 -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem &
}

servers_stop() {
    greenprint "Stopping mock JWT AUTH servers"
    local KILL_PIDS=()
    # shellcheck disable=SC2207
    # The split is desired and should be simple enough for the shell to handle
    KILL_PIDS=($(pgrep -f '^sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider'))
    for PID in "${KILL_PIDS[@]}"; do
        sudo pkill -P "$PID"
    done
}

case "$1" in
    "start")
        servers_start
        ;;
    "stop")
        servers_stop
        ;;
    *)
        echo "Usage: $0 {start|stop}"
        exit 1
        ;;
esac
