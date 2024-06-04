#!/bin/bash

#
# test osbuild-composer when the host system doesn't have redhat.repo file,
# but there is a certificate that can be used to access the repositories
#

set -exuo pipefail

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
        case "${VERSION_ID%.*}" in
            "8" | "9")
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
        echo "$0 should be skipped on ${ID}-${VERSION_ID} check gitlab-ci.yml"
        exit 1
esac

function cleanup {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # Make the cleanup function best effort
    set +eu

    greenprint "Display httpd logs"
    cat /var/log/httpd/access_log
    cat /var/log/httpd/error_log

    greenprint "Putting things back to their previous configuration"
    
    sudo mv "${REPOSITORY_OVERRIDE}.backup" "${REPOSITORY_OVERRIDE}" || echo "no repo override backup"
    if [[ -d /etc/httpd/conf.d.backup ]];
    then
        sudo rm -rf /etc/httpd/conf.d
        sudo mv /etc/httpd/conf.d.backup /etc/httpd/conf.d
    fi

    if [[ -n "${REDHAT_REPO_BACKUP:-}" ]]; then
	sudo rm -f "${REDHAT_REPO}"
	sudo mv "${REDHAT_REPO_BACKUP}" "${REDHAT_REPO}"
    fi

    sudo rm -f /etc/httpd/conf.d/repo1.conf
    sudo rm -f /etc/httpd/conf.d/repo2.conf
    sudo systemctl stop httpd || echo "failed to stop httpd"

    sudo rm -f "${REDHAT_CA_CERT}"
    sudo mv "${REDHAT_CA_CERT_BACKUP}" "${REDHAT_CA_CERT}"
    sudo rm -rf "${ENTITLEMENTS_DIR}"
    sudo mv "${ENTITLEMENTS_DIR_BACKUP}" "${ENTITLEMENTS_DIR}"
    
    set -eu
}

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

# Ensure red hat redhat.repo does not exist
REDHAT_REPO=/etc/yum.repos.d/redhat.repo
ls -l /etc/yum.repos.d/
if [ -f "${REDHAT_REPO}" ];
then
    echo "The ${REDHAT_REPO} file shouldn't exist, removing it for the test"
    REDHAT_REPO_BACKUP="${REDHAT_REPO}.backup"
    sudo chattr -i ${REDHAT_REPO}
    sudo mv "${REDHAT_REPO}" "${REDHAT_REPO_BACKUP}"
fi

REDHAT_CA_CERT="/etc/rhsm/ca/redhat-uep.pem"
REDHAT_CA_CERT_BACKUP="${REDHAT_CA_CERT}.backup"
if [ -f "${REDHAT_CA_CERT}" ];
then
    sudo mv "${REDHAT_CA_CERT}" "${REDHAT_CA_CERT_BACKUP}"
fi

# Make sure the directory exists
sudo mkdir -p /etc/rhsm/ca
# Copy the test CA cert instead of the official RH one
sudo cp "${PKI_DIR}/ca1/ca.crt" "${REDHAT_CA_CERT}"

# Make sure the directory with entitlements is empty
ENTITLEMENTS_DIR="/etc/pki/entitlement"
ENTITLEMENTS_DIR_BACKUP="${ENTITLEMENTS_DIR}.backup"
if [ -d "${ENTITLEMENTS_DIR}" ];
then
    sudo mv "${ENTITLEMENTS_DIR}" "${ENTITLEMENTS_DIR_BACKUP}"
fi
sudo mkdir -p "${ENTITLEMENTS_DIR}"

# Create the very first file to be encountered by the fallback mechanism
CLIENT_KEY="/etc/pki/entitlement/0-key.pem"
CLIENT_CERT="/etc/pki/entitlement/0.pem"
sudo cp "${PKI_DIR}/ca1/client.key" "${CLIENT_KEY}"
sudo cp "${PKI_DIR}/ca1/client.crt" "${CLIENT_CERT}"

sudo update-ca-trust

# Allow httpd process to create network connections
sudo setsebool httpd_can_network_connect on

# Reconfigure the proxies to use only a single CA
sudo sed -i "s|${PKI_DIR}/ca2|${PKI_DIR}/ca1|" /etc/httpd/conf.d/repo2.conf
sudo systemctl restart httpd
sleep 5
sudo systemctl status httpd

sudo systemctl restart osbuild-composer
sleep 5
sudo systemctl status osbuild-composer

try_image_build
