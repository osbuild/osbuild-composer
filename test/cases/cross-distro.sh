#!/usr/bin/bash

#
# Test the available distributions. Only allow releases for the current distro.
#

APISOCKET=/run/weldr/api.socket

source /etc/os-release

# Build a grep pattern that results in an empty string when the expected distros are installed
case $ID in
    fedora)
        PATTERN="\[|\]|fedora-"
        ;;
    rhel)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        case $MAJOR in
            8)
                # RHEL 8 only supports building RHEL 8
                PATTERN="\[|\]|rhel-$MAJOR"
                ;;
            *)
                # RHEL 9 and later support building all releases
                PATTERN="\[|\]|rhel-*"
                ;;
        esac
        ;;
    centos)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        case $MAJOR in
            8)
                # CentOS 8 only supports building CentosOS 8
                PATTERN="\[|\]|centos-$MAJOR"
                ;;
            *)
                # CentOS 9 and later support building all releases
                PATTERN="\[|\]|centos-*"
                ;;
        esac
        ;;
    *)
        echo "Unknown distribution id: $ID 😢"
        exit 1
    ;;
esac


# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh
echo "====> Finished Provisioning system"
echo "====> Starting $(basename "$0")"

# Remove repo overrides installed by provision.sh, these will show up in the
# list and cause it to fail and are not needed since this test doesn't build
# anything.
sudo rm -f /etc/osbuild-composer/repositories/*
sudo systemctl try-restart osbuild-composer

echo "Repository directories:"
ls -lR /etc/osbuild-composer/repositories/
ls -lR /usr/share/osbuild-composer/repositories/

echo "Repositories installed by the rpm:"
rpm -qil osbuild-composer-core

# composer-cli in RHEL 8 doesn't support distro command, so use curl for this test
if [ ! -e $APISOCKET ]; then
    echo "osbuild-composer.socket has not been started. 😢"
    exit 1
fi

if ! sudo curl -s --unix-socket $APISOCKET http:///localhost/api/status > /dev/null; then
    echo "osbuild-composer server not available. 😢"
    exit 1
fi

if ! DISTROS=$(sudo curl -s --unix-socket $APISOCKET http:///localhost/api/v1/distros/list); then
    echo "osbuild-composer server error getting distros list. 😢"
    exit 1
fi

REMAINDER=$(echo "$DISTROS" | jq -r '.distros' | grep -v -E "$PATTERN")
if [ -n "$REMAINDER" ]; then
    echo "🔥 Unexpected distros installed:"
    echo "$REMAINDER"
    exit 1
else
    echo "🎉 All tests passed."
    exit 0
fi
