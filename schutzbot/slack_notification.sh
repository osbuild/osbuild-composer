#!/bin/bash

set -eux

COMPOSE_ID=$(cat COMPOSE_ID)
MESSAGE="\"Nightly pipeline execution on *$COMPOSE_ID* finished with status *$1* $2 \n QE: @atodorov, @jrusz, @jabia \n Link to results: $CI_PIPELINE_URL \""

curl \
    -X POST \
    -H 'Content-type: application/json' \
    --data '{"text": "test", "blocks": [ { "type": "section", "text": {"type": "mrkdwn", "text":'"$MESSAGE"'}}]}' \
    "$SLACK_WEBHOOK_URL"
