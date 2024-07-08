#!/bin/bash

# This test case verifies that a repository can be configured with the
# `ignore_ssl` parameter and that osbuild downloads packages from that
# repository using the `--insecure` curl flag.

set -xeuo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

greenprint "Registering clean ups"
kill_pids=()
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    set +eu
    greenprint "Stopping containers"
    sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

    greenprint "Removing generated CA cert"

    for pid in "${kill_pids[@]}"; do
        sudo pkill -P "${pid}"
    done
    set -eu
}
trap cleanup EXIT

greenprint "Generating SSL certificate for custom repo"
certdir=$(mktemp -d)
certfile="${certdir}/certificate.pem"
keyfile="${certdir}/key.pem"
openssl req -new -newkey rsa:4096 -days 1 -nodes -x509 \
    -subj "/C=DE/ST=Berlin/L=Berlin/O=Org/CN=osbuild.org" \
    -keyout "${keyfile}" -out "${certfile}"

greenprint "Generate GPG key to sign custom RPMs are repo metadata"
gpg --batch --passphrase '' --quick-gen-key testing@redhat.com default default
gpg_pubkey=$(gpg --armor --export testing@redhat.com)

cat <<EOF > ~/.rpmmacros
%_gpg_name testing@redhat.com
EOF


greenprint "Creating dummy rpm and repo"
# make a dummy rpm and repo to test payload_repositories
sudo dnf install -y rpm-build rpm-sign createrepo
dummyrpmdir=$(mktemp -d)
dummyspecfile="$dummyrpmdir/dummy.spec"

cat <<EOF > "$dummyspecfile"
#----------- spec file starts ---------------
Name:                   dummy
Version:                1.0.0
Release:                0
BuildArch:              noarch
Vendor:                 dummy
Summary:                Provides %{name}
License:                BSD
Provides:               dummy

%description
%{summary}

%files
EOF

mkdir -p "DUMMYRPMDIR/rpmbuild"
rpmbuild --quiet --define "_topdir $dummyrpmdir/rpmbuild" -bb "$dummyspecfile"
rpmsign --addsign "${dummyrpmdir}"/rpmbuild/RPMS/noarch/*.rpm
# debug
sha256sum "${dummyrpmdir}"/rpmbuild/RPMS/noarch/*.rpm

mkdir -p "${dummyrpmdir}/repo"
cp "${dummyrpmdir}"/rpmbuild/RPMS/noarch/*rpm "$dummyrpmdir/repo"
createrepo "${dummyrpmdir}/repo"

greenprint "Creating web service with TLS to serve repo"
sudo dnf install -y go
websrvdir=$(mktemp -d)
websrvfile="${websrvdir}/serve.go"
websrvport=4430
websrvurl="https://localhost:${websrvport}"
cat <<EOF > "${websrvfile}"
package main

import (
	"log"
	"net/http"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir("${dummyrpmdir}/repo")))
	log.Fatal(http.ListenAndServeTLS(":${websrvport}", "${certfile}", "${keyfile}", nil))
}
EOF

go run "${websrvfile}" &
kill_pids+=("$!")

COUNTER=0
# make sure the repository is already accessible
while [[ "$(curl -k -s -o /dev/null -w '%{http_code}' https://localhost:${websrvport}/repodata/repomd.xml)" != "200" && $COUNTER -lt 10 ]]; do
    greenprint "$COUNTER - Not ready yet ....."
    (( COUNTER=COUNTER+1 ))
    sleep 60
done

greenprint "Creating source file and blueprint"
composedir=$(mktemp -d)
blueprint="${composedir}/blueprint.toml"
dummysource="${composedir}/dummy.toml"
composestart="${composedir}/compose-start.json"
composeinfo="${composedir}/compose-info.json"
modulesinfo="${composedir}/modules-info.json"

# Write a source (repo) config to add to composer
cat <<EOF > "${dummysource}"
id = "test"
name = "test repository"
type = "yum-baseurl"
url = "${websrvurl}"
rhsm = false
check_gpg = true
gpgkeys = ['''${gpg_pubkey}''']
check_ssl = false
EOF

sudo composer-cli sources add "${dummysource}"
sudo composer-cli sources info test

# Write a basic blueprint for our image.
cat <<EOF > "${blueprint}"
name = "dummy"
description = "A base system with the dummy package"
version = "0.0.1"

[[packages]]
name = "dummy"
EOF

sudo composer-cli blueprints push "${blueprint}"
sudo composer-cli blueprints depsolve dummy

# confirm dummy is fetched from the custom repo
sudo composer-cli modules info --json dummy | tee "${modulesinfo}"
dummysourceurl=$(get_build_info '.modules[0].dependencies[0].remote_location' "${modulesinfo}")
expectedurl="${websrvurl}/dummy-1.0.0-0.noarch.rpm"
if [[ "${dummysourceurl}" != "${expectedurl}" ]]; then
    echo "Unexpected package URL: ${dummysourceurl}"
    echo "Expected:               ${expectedurl}"
    exit 1
fi

sudo composer-cli --json compose start dummy qcow2 | tee "${composestart}"
composeid=$(get_build_info '.build_id' "${composestart}")

# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${composeid}"
while true; do
    sudo composer-cli --json compose info "${composeid}" | tee "${composeinfo}" > /dev/null
    composestatus=$(get_build_info '.queue_status' "${composeinfo}")

    # Is the compose finished?
    if [[ ${composestatus} != RUNNING ]] && [[ ${composestatus} != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

sudo composer-cli compose delete "${composeid}" >/dev/null

jq . "${composeinfo}"

# Did the compose finish with success?
if [[ $composestatus == FINISHED ]]; then
    echo "Test passed!"
    exit 0
else
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
