#!/bin/bash

set -eux

if [ -z "${SLACK_WEBHOOK_URL:-}" ]; then
    echo "INFO: Variable SLACK_WEBHOOK_URL is undefined"
    exit 0
fi

if [ "$3" == "ga" ]; then
    MESSAGE="\"<$CI_PIPELINE_URL|GA composes pipeline>: *$1* $2, cc <@U01CUGX9L68>, <@U01Q07AHZ9C>, <@U04PYMDRV5H>\""
else
    COMPOSE_ID=$(cat COMPOSE_ID)
    COMPOSER_NVR=$(cat COMPOSER_NVR)
    MESSAGE="\"<$CI_PIPELINE_URL|Nightly pipeline> ($COMPOSE_ID: $COMPOSER_NVR): *$1* $2, cc <@U01CUGX9L68>, <@U01Q07AHZ9C>, <@U04PYMDRV5H>\""
fi

curl \
    -X POST \
    -H 'Content-type: application/json' \
    --data '{"text": '"$MESSAGE"'}' \
    "$SLACK_WEBHOOK_URL"
