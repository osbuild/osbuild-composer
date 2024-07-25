#!/bin/bash

#
# osbuild-composer can work with multiple subscriptions on the host system.
# test this by simulating a custom repository with custom certificates and 
# verifies that an image can be built using those.
#

set -exuo pipefail

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function generate_certificates {
    # Generate CA root key
    sudo openssl genrsa -out ca.key
    # Create and self-sign root certificate
    sudo openssl req -new -subj "/C=GB/CN=ca" -addext "subjectAltName = DNS:localhost" -key ca.key -out ca.csr
    sudo openssl x509 -req -sha256 -days 365 -in ca.csr -signkey ca.key -out ca.crt
    # Key for the server
    sudo openssl genrsa -out server.key
    # Certificate for the server
    sudo openssl req -new -subj "/C=GB/CN=localhost" -sha256 -key server.key -out server.csr
    sudo openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -sha256
    # Key for the client
    sudo openssl genrsa -out client.key
    # Certificate for the client
    sudo openssl req -new -subj "/C=GB/CN=localhost" -sha256 -key client.key -out client.csr
    sudo openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256

    # add the certificate authority to the system trust stores
    sudo cp ca.crt "/etc/pki/ca-trust/source/anchors/ca-$(uuidgen).crt"
    sudo update-ca-trust
}

function cleanup {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # Make the cleanup function best effort
    set +eu

    greenprint "Display httpd logs"
    sudo cat /var/log/httpd/access_log
    sudo cat /var/log/httpd/error_log

    greenprint "Putting things back to their previous configuration"
    if [ -n "${REDHAT_REPO}" ] && [ -n "${REDHAT_REPO_BACKUP}" ];
    then
        lsattr "${REDHAT_REPO}"
        sudo chattr -i "${REDHAT_REPO}"
        sudo rm -f "${REDHAT_REPO}"
        sudo mv "${REDHAT_REPO_BACKUP}" "${REDHAT_REPO}" || echo "no redhat.repo backup"
        sudo mv "${REPOSITORY_OVERRIDE}.backup" "${REPOSITORY_OVERRIDE}" || echo "no repo override backup"
    fi
    if [[ -d /etc/httpd/conf.d.backup ]];
    then
        sudo rm -rf /etc/httpd/conf.d
        sudo mv /etc/httpd/conf.d.backup /etc/httpd/conf.d
    fi
    sudo rm -f /etc/httpd/conf.d/repo1.conf
    sudo rm -f /etc/httpd/conf.d/repo2.conf
    sudo systemctl stop httpd || echo "failed to stop httpd"
}

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh none

# Discover what system is installed on the runner
case "${ID}" in
    "fedora")
        echo "Running on Fedora"
        DISTRO_NAME="${ID}-${VERSION_ID}"
        REPOSITORY_OVERRIDE="/etc/osbuild-composer/repositories/${DISTRO_NAME}.json"
        REPO1_NAME="fedora"
        REPO2_NAME="updates"
        ;;
    "rhel")
        echo "Running on RHEL"

        if [[ "$VERSION_ID" == "9.5" || "$VERSION_ID" == "10.0" ]]; then
            # fails eventhough we call update-ca-trust, see previous commit
            echo "This test has been disabled b/c DNF fails with self-signed certificates"
            exit 0
        fi

        case "${VERSION_ID%.*}" in
            "8" | "9" | "10")
                echo "Running on RHEL ${VERSION_ID}"
                # TODO: remove once the osbuild-composer v100 is in RHEL
                if ! nvrGreaterOrEqual "osbuild-composer" "100"; then
                    VERSION_SUFFIX=$(echo "${VERSION_ID}" | tr -d ".")
                else
                    VERSION_SUFFIX=${VERSION_ID}
                fi
                DISTRO_NAME="${ID}-${VERSION_SUFFIX}"
                REPOSITORY_OVERRIDE="/etc/osbuild-composer/repositories/${DISTRO_NAME}.json"
                REPO1_NAME="baseos"
                REPO2_NAME="appstream"
                ;;
            *)
                echo "Unknown RHEL: ${VERSION_ID}"
                exit 1
        esac
        ;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1
esac

trap cleanup EXIT

# If the runner doesn't use overrides, start using it.
if [ ! -f "${REPOSITORY_OVERRIDE}" ];
then
    REPODIR=/etc/osbuild-composer/repositories/
    sudo mkdir -p "${REPODIR}"
    sudo cp "/usr/share/tests/osbuild-composer/repositories/${DISTRO_NAME}.json" "${REPOSITORY_OVERRIDE}"
fi

# Configuration of the testing environment
REPO1_BASEURL=$(jq --raw-output ".${ARCH}[] | select(.name==\"${REPO1_NAME}\") | .baseurl" "${REPOSITORY_OVERRIDE}")
REPO2_BASEURL=$(jq --raw-output ".${ARCH}[] | select(.name==\"${REPO2_NAME}\") | .baseurl" "${REPOSITORY_OVERRIDE}")

# Don't use raw-output, instead cut the surrounding quotes to preserve \n inside the string so that it can be
# easily written to files using "tee" later in the script.
REPO1_GPGKEY=$(jq ".${ARCH}[] | select(.name==\"${REPO1_NAME}\") | .gpgkey" "${REPOSITORY_OVERRIDE}" | cut -d'"' -f2)
REPO2_GPGKEY=$(jq ".${ARCH}[] | select(.name==\"${REPO2_NAME}\") | .gpgkey" "${REPOSITORY_OVERRIDE}" | cut -d'"' -f2)

# RPMrepo tool uses redirects to different AWS S3 buckets, VPCs or in case of PSI a completely different redirect.
# Dynamically discover the URL that the repos redirect to.
PROXY1_REDIRECT_URL=$(curl -s -o /dev/null -w '%{redirect_url}' "${REPO1_BASEURL}"repodata/repomd.xml | cut -f1,2,3 -d'/')/
PROXY2_REDIRECT_URL=$(curl -s -o /dev/null -w '%{redirect_url}' "${REPO2_BASEURL}"repodata/repomd.xml | cut -f1,2,3 -d'/')/

# Some repos, e.g. the internal mirrors don't have any redirections, if that happens, just put a placeholder into the variable.
if [[ "${PROXY1_REDIRECT_URL}" == "/" ]];
then
    PROXY1_REDIRECT_URL="http://example.com/"
fi
if [[ "${PROXY2_REDIRECT_URL}" == "/" ]];
then
    PROXY2_REDIRECT_URL="http://example.com/"
fi

PKI_DIR=/etc/pki/httpd

greenprint "Creating certification authorities"

sudo mkdir -p "${PKI_DIR}/ca1"
sudo mkdir -p "${PKI_DIR}/ca2"

pushd "${PKI_DIR}/ca1"
generate_certificates
popd

pushd "${PKI_DIR}/ca2"
generate_certificates
popd

# osbuild-composer will need to read this even when not running as root
sudo chmod +r /etc/pki/httpd/ca1/*.key
sudo chmod +r /etc/pki/httpd/ca2/*.key

greenprint "Initialize httpd configurations"
sudo mv /etc/httpd/conf.d /etc/httpd/conf.d.backup
sudo mkdir -p /etc/httpd/conf.d
sudo tee /etc/httpd/conf.d/repo1.conf << STOPHERE
# Port to Listen on
Listen 8008

<VirtualHost *:8008>
   # Just pass all the requests to the real mirror
   ProxyPass /repo/ ${REPO1_BASEURL}
   ProxyPassReverse /repo/ ${REPO1_BASEURL}
   # The real mirror redirects to this URL, so proxy this one as well, otherwise
   # it won't work with the self-signed client certificates
   ProxyPass /aws/ ${PROXY1_REDIRECT_URL}
   ProxyPassReverse /aws/ ${PROXY1_REDIRECT_URL}
   # But turn on SSL
   SSLEngine on
   SSLProxyEngine on
   SSLCertificateFile ${PKI_DIR}/ca1/server.crt
   SSLCertificateKeyFile ${PKI_DIR}/ca1/server.key
   # And require the client to authenticate using a certificate issued by our custom CA
   SSLVerifyClient require
   SSLVerifyDepth 1
   SSLCACertificateFile ${PKI_DIR}/ca1/ca.crt
</VirtualHost>
STOPHERE

sudo tee /etc/httpd/conf.d/repo2.conf << STOPHERE
# Port to Listen on
Listen 8009

<VirtualHost *:8009>
   # Just pass all the requests to the real mirror
   ProxyPass /repo/ ${REPO2_BASEURL}
   ProxyPassReverse /repo/ ${REPO2_BASEURL}
   # The real mirror redirects to this URL, so proxy this one as well, otherwise
   # it won't work with the self-signed client certificates
   ProxyPass /aws/ ${PROXY2_REDIRECT_URL}
   ProxyPassReverse /aws/ ${PROXY2_REDIRECT_URL}
   # But turn on SSL
   SSLEngine on
   SSLProxyEngine on
   SSLCertificateFile ${PKI_DIR}/ca2/server.crt
   SSLCertificateKeyFile ${PKI_DIR}/ca2/server.key
   # And require the client to authenticate using a certificate issued by our custom CA
   SSLVerifyClient require
   SSLVerifyDepth 1
   SSLCACertificateFile ${PKI_DIR}/ca2/ca.crt
</VirtualHost>
STOPHERE

REDHAT_REPO=/etc/yum.repos.d/redhat.repo
REDHAT_REPO_BACKUP=/etc/yum.repos.d/redhat.repo.backup
sudo mv ${REDHAT_REPO} ${REDHAT_REPO_BACKUP} || echo "no redhat.repo"
sudo tee ${REDHAT_REPO} << STOPHERE
[repo1]
name = Repo 1 - local proxy
baseurl = https://localhost:8008/repo
enabled = 1
gpgcheck = 0
sslverify = 1
sslcacert = ${PKI_DIR}/ca1/ca.crt
sslclientkey = ${PKI_DIR}/ca1/client.key
sslclientcert = ${PKI_DIR}/ca1/client.crt
metadata_expire = 86400
enabled_metadata = 0

[repo2]
name = Repo 2 - local proxy
baseurl = https://localhost:8009/repo
enabled = 1
gpgcheck = 0
sslverify = 1
sslcacert = ${PKI_DIR}/ca2/ca.crt
sslclientkey = ${PKI_DIR}/ca2/client.key
sslclientcert = ${PKI_DIR}/ca2/client.crt
metadata_expire = 86400
enabled_metadata = 0
STOPHERE

sudo chattr +i ${REDHAT_REPO}
lsattr ${REDHAT_REPO}
cat ${REDHAT_REPO}

# Allow httpd process to create network connections
sudo setsebool httpd_can_network_connect on
# Start httpd
sudo systemctl start httpd || echo "Starting httpd failed"
sudo systemctl status httpd

greenprint "Verify dnf can use this configuration"
sudo dnf install --repo=repo1 --repo=repo2 zsh -y

greenprint "Rewrite osbuild-composer repository configuration"
# In case this test case runs as part of multiple different test, try not to ruit the environment
sudo mv "${REPOSITORY_OVERRIDE}" "${REPOSITORY_OVERRIDE}.backup"
sudo tee "${REPOSITORY_OVERRIDE}" << STOPHERE
{
  "${ARCH}": [
    {
      "baseurl": "https://localhost:8008/repo",
      "gpgkey": "${REPO1_GPGKEY}",
      "check_gpg": false,
      "rhsm": true
    },
    {
      "baseurl": "https://localhost:8009/repo",
      "gpgkey": "${REPO2_GPGKEY}",
      "check_gpg": false,
      "rhsm": true
    }
  ]
}
STOPHERE

sudo systemctl restart osbuild-composer
sudo composer-cli status show

BLUEPRINT_FILE=/tmp/bp.toml
BLUEPRINT_NAME=zishy

cat > "$BLUEPRINT_FILE" << STOPHERE
name = "${BLUEPRINT_NAME}"
description = "A base system with zsh"
version = "0.0.1"

[[packages]]
name = "zsh"
STOPHERE

function try_image_build {
    COMPOSE_START=/tmp/compose-start.json
    COMPOSE_INFO=/tmp/compose-info.json
    sudo composer-cli blueprints push "$BLUEPRINT_FILE"
    if ! sudo composer-cli blueprints depsolve ${BLUEPRINT_NAME};
    then
        sudo cat /var/log/httpd/error_log
        sudo journalctl -xe --unit osbuild-composer
        exit 1
    fi
    if ! sudo composer-cli --json compose start ${BLUEPRINT_NAME} qcow2 | tee "${COMPOSE_START}";
    then
        sudo journalctl -xe --unit osbuild-composer
        WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
        sudo journalctl -xe --unit "${WORKER_UNIT}"
        exit 1
    fi
    COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "${COMPOSE_INFO}" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 30

    done

    sudo composer-cli compose delete "${COMPOSE_ID}" >/dev/null

    sudo journalctl -xe --unit osbuild-composer
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -xe --unit "${WORKER_UNIT}"

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS == FINISHED ]]; then
        echo "Test passed!"
        exit 0
    else
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

try_image_build
