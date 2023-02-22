#!/bin/bash
set -euxo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# create artifacts folder
ARTIFACTS="${ARTIFACTS:=/tmp/artifacts}"
mkdir -p "${ARTIFACTS}"

# determine the authentication method used by composer
AUTH_METHOD_TLS="tls"
AUTH_METHOD_JWT="jwt"
AUTH_METHOD_NONE="none"
# default to TLS for now
AUTH_METHOD="${1:-$AUTH_METHOD_TLS}"

# koji and ansible are not in RHEL repositories. Depending on them in the spec
# file breaks RHEL gating (see OSCI-1541). Therefore, we need to enable epel
# and install koji and ansible here.
if [[ $ID == rhel || $ID == centos ]] && ! rpm -q epel-release; then
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-"${VERSION_ID%.*}".noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

sudo dnf install -y ansible-core koji

# workaround for bug https://bugzilla.redhat.com/show_bug.cgi?id=2057769
if [[ "$VERSION_ID" == "9.0" || "$VERSION_ID" == "9" ]]; then
    if [[ -f "/usr/share/qemu/firmware/50-edk2-ovmf-amdsev.json" ]]; then
        jq '.mapping += {"nvram-template": {"filename": "/usr/share/edk2/ovmf/OVMF_VARS.fd","format": "raw"}}' /usr/share/qemu/firmware/50-edk2-ovmf-amdsev.json | sudo tee /tmp/50-edk2-ovmf-amdsev.json
        sudo mv /tmp/50-edk2-ovmf-amdsev.json /usr/share/qemu/firmware/50-edk2-ovmf-amdsev.json
    fi
fi

sudo mkdir -p /etc/osbuild-composer
sudo mkdir -p /etc/osbuild-worker

# osbuild-composer and worker need to be configured in a specific way only when using
# some authentication method (Service scenario). In such case, also credentials for
# interacting with cloud providers are configured directly in the worker. In addition,
# no certificates need to be generated, because they are not used anywhere in this
# scenario.
if [[ "$AUTH_METHOD" != "$AUTH_METHOD_NONE" ]]; then
    # Generate all X.509 certificates for the tests
    # The whole generation is done in a $CADIR to better represent how osbuild-ca
    # it.
    CERTDIR=/etc/osbuild-composer
    OPENSSL_CONFIG=/usr/share/tests/osbuild-composer/x509/openssl.cnf
    CADIR=/etc/osbuild-composer-test/ca

    scriptloc=$(dirname "$0")
    sudo "${scriptloc}/gen-certs.sh" "${OPENSSL_CONFIG}" "${CERTDIR}" "${CADIR}"
    sudo chown _osbuild-composer "${CERTDIR}"/composer-*.pem

    # Copy the appropriate configuration files
    if [[ "$AUTH_METHOD" == "$AUTH_METHOD_JWT" ]]; then
        COMPOSER_TEST_CONFIG="/usr/share/tests/osbuild-composer/composer/osbuild-composer-jwt.toml"
        WORKER_TEST_CONFIG="/usr/share/tests/osbuild-composer/worker/osbuild-worker-jwt.toml"

        # Default orgID
        sudo tee "/etc/osbuild-worker/token" >/dev/null <<EOF
123456789
EOF

        /usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh start

    elif [[ "$AUTH_METHOD" == "$AUTH_METHOD_TLS" ]]; then
        COMPOSER_TEST_CONFIG="/usr/share/tests/osbuild-composer/composer/osbuild-composer-tls.toml"
        WORKER_TEST_CONFIG="/usr/share/tests/osbuild-composer/worker/osbuild-worker-tls.toml"
    fi

    sudo cp -a "$COMPOSER_TEST_CONFIG" /etc/osbuild-composer/osbuild-composer.toml
    sudo cp -a "$WORKER_TEST_CONFIG" /etc/osbuild-worker/osbuild-worker.toml

    # if GCP credentials are defined in the ENV, add them to the worker's configuration
    GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-}"
    if [ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
        # The credentials file must be copied to a different location. Jenkins places
        # it into /tmp and as a result, the worker would not see it due to using PrivateTmp=true.
        GCP_CREDS_WORKER_PATH="/etc/osbuild-worker/gcp-credentials.json"
        sudo cp "$GOOGLE_APPLICATION_CREDENTIALS" "$GCP_CREDS_WORKER_PATH"
        sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[gcp]
credentials = "$GCP_CREDS_WORKER_PATH"
bucket = "$GCP_BUCKET"
EOF
    fi

    # if Azure credentials are defined in the env, create the credentials file
    V2_AZURE_CLIENT_ID="${V2_AZURE_CLIENT_ID:-}"
    V2_AZURE_CLIENT_SECRET="${V2_AZURE_CLIENT_SECRET:-}"
    if [[ -n "$V2_AZURE_CLIENT_ID" && -n "$V2_AZURE_CLIENT_SECRET" ]]; then
        set +x
        sudo tee /etc/osbuild-worker/azure-credentials.toml > /dev/null << EOF
client_id =     "$V2_AZURE_CLIENT_ID"
client_secret = "$V2_AZURE_CLIENT_SECRET"
EOF
        sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[azure]
credentials = "/etc/osbuild-worker/azure-credentials.toml"
EOF
        set -x
    fi

    # if AWS credentials are defined in the ENV, add them to the worker's configuration
    V2_AWS_ACCESS_KEY_ID="${V2_AWS_ACCESS_KEY_ID:-}"
    V2_AWS_SECRET_ACCESS_KEY="${V2_AWS_SECRET_ACCESS_KEY:-}"
    if [[ -n "$V2_AWS_ACCESS_KEY_ID" && -n "$V2_AWS_SECRET_ACCESS_KEY" ]]; then
        set +x
    sudo tee /etc/osbuild-worker/aws-credentials.toml > /dev/null << EOF
[default]
aws_access_key_id = "$V2_AWS_ACCESS_KEY_ID"
aws_secret_access_key = "$V2_AWS_SECRET_ACCESS_KEY"
EOF
        sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[aws]
credentials = "/etc/osbuild-worker/aws-credentials.toml"
bucket = "${AWS_BUCKET}"
EOF
        set -x
    fi

else # AUTH_METHOD_NONE
    # Repositories in /etc/osbuild-composer/repositories are used only in the
    # on-premise scenario (Weldr).
    # Copy rpmrepo snapshots for use in weldr tests
    REPODIR=/etc/osbuild-composer/repositories
    sudo mkdir -p $REPODIR
    # Copy all fedora repo overrides
    sudo cp -a /usr/share/tests/osbuild-composer/repositories/{fedora,centos}-*.json "$REPODIR"
    # Copy RHEL point release repos
    sudo cp -a /usr/share/tests/osbuild-composer/repositories/rhel-*.json "$REPODIR"

    # override source repositories to consume content from the nightly compose
    if [ "${NIGHTLY:=false}" == "true" ]; then
        source /usr/libexec/osbuild-composer-test/define-compose-url.sh

        VERSION_SUFFIX=$(echo "${VERSION_ID}" | tr -d ".")
        for ARCH in aarch64 ppc64le s390x x86_64; do
            for REPO_NAME in BaseOS AppStream RT; do
                REPO_NAME_LOWERCASE=$(echo "$REPO_NAME" | tr "[:upper:]" "[:lower:]")
                # will replace only the lines which match
                sudo sed -i "s|https://rpmrepo.osbuild.org/v2/mirror/rhvpn/el.*${ARCH}-${REPO_NAME_LOWERCASE}-.*|${COMPOSE_URL}/compose/${REPO_NAME}/${ARCH}/os/\",|" "/etc/osbuild-composer/repositories/rhel-${VERSION_SUFFIX}.json"
            done
        done
    fi
fi

# start appropriate units
case "${AUTH_METHOD}" in
    "${AUTH_METHOD_JWT}" | "${AUTH_METHOD_TLS}")
        # JWT / TLS are used only in the "Service" scenario. This means that:
        # - only remote workers will be used (no local worker)
        # - only Cloud API socket will be started (no Weldr API)
        sudo systemctl stop 'osbuild*'
        # make sure that the local worker is not running
        sudo systemctl mask osbuild-worker@1.service
        # enable remote worker API
        sudo systemctl start osbuild-remote-worker.socket
        # enable Cloud API
        sudo systemctl start osbuild-composer-api.socket
        # start a remote worker
        sudo systemctl start osbuild-remote-worker@localhost:8700.service
        ;;

    "${AUTH_METHOD_NONE}")
        # No authentication method is used on-premise with Weldr. This means that:
        # - only local worker will be used (started automatically)
        # - only Weldr API socket will be started
        sudo systemctl stop 'osbuild*'
        # enable Weldr API
        sudo systemctl start osbuild-composer.socket

        # Print debugging info about content sources
        sudo composer-cli status show
        sudo composer-cli sources list
        for SOURCE in $(sudo composer-cli sources list); do
            sudo composer-cli sources info "$SOURCE"
        done
        ;;
esac
