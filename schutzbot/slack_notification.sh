#!/bin/bash

set -eu

if [ -z "${SLACK_WEBHOOK_URL:-}" ]; then
    echo "INFO: Variable SLACK_WEBHOOK_URL is undefined"
    exit 0
fi

if [ "$3" == "ga" ]; then
    if [ "$1" == "FAILED" ]; then
      MESSAGE="\"<$CI_PIPELINE_URL|GA composes pipeline>: *$1* $2, cc <@U01S1KWFMFF>, <@U04PYMDRV5H>\""
    else
      MESSAGE="\"<$CI_PIPELINE_URL|GA composes pipeline>: *$1* $2\""
    fi
else
    COMPOSE_ID=$(cat COMPOSE_ID)
    COMPOSER_NVR=$(cat COMPOSER_NVR)
    if [ "$1" == "FAILED" ]; then
      MESSAGE="\"<$CI_PIPELINE_URL|Nightly pipeline> ($COMPOSE_ID: $COMPOSER_NVR): *$1* $2, cc <@U01S1KWFMFF>, <@U04PYMDRV5H>\""
    else
      MESSAGE="\"<$CI_PIPELINE_URL|Nightly pipeline> ($COMPOSE_ID: $COMPOSER_NVR): *$1* $2\""
    fi
fi

echo "INFO: Sending slack notification: $MESSAGE"

curl \
    -X POST \
    -H 'Content-type: application/json' \
    --data '{"text": '"$MESSAGE"'}' \
    "$SLACK_WEBHOOK_URL"
