#!/bin/bash
# https://bugzilla.redhat.com/show_bug.cgi?id=2056451

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

case "${ID}-${VERSION_ID}" in
    "rhel-8.6" | "rhel-9.0" | "centos-9" | "centos-8")
        ;;
    *)
        echo "$0 is not enabled for ${ID}-${VERSION_ID} skipping..."
        exit 0
        ;;
esac

if [ "$ARCH" != "x86_64" ]; then
  echo "Workstation group is only available on x86_64"
  exit 0
fi


set -xeuo pipefail

function get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
}

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none
BLUEPRINT_FILE=/tmp/blueprint.toml
COMPOSE_START=/tmp/compose-start.json
COMPOSE_INFO=/tmp/compose-info.json

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << 'EOF'
name = "toobig"
description = "too big blueprint"
version = "0.0.1"
modules = []
#groups = []

[[customizations.user]]
# password for admin is rootroot
name = "admin"
description = "admin"
password = "$6$ismFu3TUg0KR8.kJ$rddx3JVWXVaPF06XHeS1QNV6D6U3vo8WN4mi/V2mKLZ9ZKsMUlIwLhU.WvxfT.5F1PqUrx8Y8DUr/a5iTJQlw."
home = "/home/admin/"
shell = "/usr/bin/bash"
groups = ["users", "wheel"]

[[groups]]
name="Workstation"

[[packages]]
name="httpd"
version="*"

[[packages]]
name="gnome-session"
version="*"

[customizations]
hostname = "custombase"
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve toobig
sudo composer-cli --json compose start toobig image-installer | tee "${COMPOSE_START}"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

sudo composer-cli compose delete "${COMPOSE_ID}" >/dev/null

jq . "${COMPOSE_INFO}"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS == FINISHED ]]; then
    echo "Test passed!"
    exit 0
else
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
