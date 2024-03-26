#!/bin/bash

APP="/usr/libexec/osbuild-composer/osbuild-worker"
APP_ARGS="${WORKER_ARGS:-}"

if [[ -n "${GODEBUG_PORT:-}" ]]; then
  echo "With golang debugger enabled on port ${GODEBUG_PORT} ..."
  echo "NOTE: you HAVE to attach the debugger NOW otherwise the osbuild-worker will not continue running"
  /usr/bin/dlv "--listen=:${GODEBUG_PORT}" --headless=true --api-version=2 exec ${APP} -- "${APP_ARGS}"
  exit $?
fi

${APP} "${APP_ARGS}"
