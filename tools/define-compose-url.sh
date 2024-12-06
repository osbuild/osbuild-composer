#!/bin/bash
set -euxo pipefail
source /etc/os-release

# This isn't needed when not running on RHEL
if [[ $ID != rhel ]]; then
  return 0
fi

# This section serves to define target repos for rhel-upgrade test
# Need to check that it's actually a number passed as this script is
# being sources from a script that already has arguments passed
if [[ $# -gt 0 ]] && [[ "$1" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
    VERSION_ID=$1
fi

if [[ $ID == rhel && ${VERSION_ID%.*} == 9 ]]; then
  COMPOSE_ID=$(curl -L http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/latest-RHEL-"${VERSION_ID}"/COMPOSE_ID)

  # default to a nightly tree but respect values passed from ENV so we can test rel-eng composes as well
  COMPOSE_URL="${COMPOSE_URL:-http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/$COMPOSE_ID}"

elif [[ $ID == rhel && ${VERSION_ID%.*} == 10 ]]; then
  COMPOSE_ID=$(curl -L http://download.devel.redhat.com/rhel-10/nightly/RHEL-10/latest-RHEL-"${VERSION_ID}"/COMPOSE_ID)

  # default to a nightly tree but respect values passed from ENV so we can test rel-eng composes as well
  COMPOSE_URL="${COMPOSE_URL:-http://download.devel.redhat.com/rhel-10/nightly/RHEL-10/$COMPOSE_ID}"
fi

# in case COMPOSE_URL was defined from the outside refresh COMPOSE_ID file,
# used for slack messages in case of success/failure
curl -L "$COMPOSE_URL/COMPOSE_ID" > COMPOSE_ID

echo "INFO: Testing COMPOSE_ID=$COMPOSE_ID at COMPOSE_URL=$COMPOSE_URL"

# Make sure the compose URL really exists
RETURN_CODE=$(curl --silent -o -I -L -s -w "%{http_code}" "${COMPOSE_URL}")
if [ "$RETURN_CODE" != 200 ]
then
    echo "Compose URL $COMPOSE_URL returned error code $RETURN_CODE, exiting."
    exit 1
fi
