#!/bin/bash
set -euo pipefail

curl -L http://download.devel.redhat.com/rhel-8/nightly/RHEL-8/latest-finished-RHEL-8.5/COMPOSE_ID > COMPOSE_ID
COMPOSE_ID=$(cat COMPOSE_ID)

# default to a nightly tree but respect values passed from ENV so we can test rel-eng composes as well
COMPOSE_URL="${COMPOSE_URL:-http://download.devel.redhat.com/rhel-8/nightly/RHEL-8/$COMPOSE_ID}"

# in case COMPOSE_URL was defined from the outside refresh COMPOSE_ID file,
# used for slack messages in case of success/failure
curl -L "$COMPOSE_URL/COMPOSE_ID" > COMPOSE_ID

# Make sure the compose URL really exists
RETURN_CODE=$(curl --silent -o -I -L -s -w "%{http_code}" "${COMPOSE_URL}")
if [ "$RETURN_CODE" != 200 ]
then
    echo "Compose URL $COMPOSE_URL returned error code $RETURN_CODE, exiting."
    exit 1
fi
