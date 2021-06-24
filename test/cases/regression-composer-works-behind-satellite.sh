#!/bin/bash

set -exuo pipefail

function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

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
}

ARCH=$(uname -m)
if [ "${ARCH}" = "x86_64" ];
then
    SNAPSHOT="20210326"
elif [ "${ARCH}" = "aarch64" ];
then
    SNAPSHOT="20210414"
else
    echo "${ARCH} architecture is not supported in rpmrepo tool. Skipping this test."
    exit 0
fi

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

greenprint "Installing dependencies"

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
   ProxyPass /repo/ https://rpmrepo.osbuild.org/v2/mirror/public/f33/f33-${ARCH}-fedora-${SNAPSHOT}/
   ProxyPassReverse /repo/ https://rpmrepo.osbuild.org/v2/mirror/public/f33/f33-${ARCH}-fedora-${SNAPSHOT}/
   # The real mirror redirects to this URL, so proxy this one as well, otherwise
   # it won't work with the self-signed client certificates
   ProxyPass /aws/ https://rpmrepo-storage.s3.amazonaws.com/
   ProxyPassReverse /aws/ https://rpmrepo-storage.s3.amazonaws.com/
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
   ProxyPass /repo/ https://rpmrepo.osbuild.org/v2/mirror/public/f33/f33-${ARCH}-fedora-modular-${SNAPSHOT}/
   ProxyPassReverse /repo/ https://rpmrepo.osbuild.org/v2/mirror/public/f33/f33-${ARCH}-fedora-modular-${SNAPSHOT}/
   # The real mirror redirects to this URL, so proxy this one as well, otherwise
   # it won't work with the self-signed client certificates
   ProxyPass /aws/ https://rpmrepo-storage.s3.amazonaws.com/
   ProxyPassReverse /aws/ https://rpmrepo-storage.s3.amazonaws.com/
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

sudo mv /etc/yum.repos.d/redhat.repo /etc/yum.repos.d/redhat.repo.backup || echo "no redhat.repo"
sudo tee /etc/yum.repos.d/redhat.repo << STOPHERE
[f33]
name = Fedora 33 - Local proxy
baseurl = https://localhost:8008/repo
enabled = 1
gpgcheck = 0
sslverify = 1
sslcacert = ${PKI_DIR}/ca1/ca.crt
sslclientkey = ${PKI_DIR}/ca1/client.key
sslclientcert = ${PKI_DIR}/ca1/client.crt
metadata_expire = 86400
enabled_metadata = 0

[f33-modular]
name = Fedora 33 Modular - Local proxy
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

# Allow httpd process to create network connections
sudo setsebool httpd_can_network_connect on
# Start httpd
sudo systemctl start httpd || echo "Starting httpd failed"
sudo systemctl status httpd

greenprint "Verify dnf can use this configuration"
sudo dnf install --repo=f33 --repo=f33-modular fish -y

greenprint "Rewrite osbuild-composer repository configuration"
# In case this test case runs as part of multiple different test, try not to ruit the environment
sudo mv /etc/osbuild-composer/repositories/fedora-33.json /etc/osbuild-composer/repositories/fedora-33.json.backup
sudo tee /etc/osbuild-composer/repositories/fedora-33.json << STOPHERE
{
  "x86_64": [
    {
      "baseurl": "https://localhost:8008/repo",
      "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBF4wBvsBEADQmcGbVUbDRUoXADReRmOOEMeydHghtKC9uRs9YNpGYZIB+bie\nbGYZmflQayfh/wEpO2W/IZfGpHPL42V7SbyvqMjwNls/fnXsCtf4LRofNK8Qd9fN\nkYargc9R7BEz/mwXKMiRQVx+DzkmqGWy2gq4iD0/mCyf5FdJCE40fOWoIGJXaOI1\nTz1vWqKwLS5T0dfmi9U4Tp/XsKOZGvN8oi5h0KmqFk7LEZr1MXarhi2Va86sgxsF\nQcZEKfu5tgD0r00vXzikoSjn3qA5JW5FW07F1pGP4bF5f9J3CZbQyOjTSWMmmfTm\n2d2BURWzaDiJN9twY2yjzkoOMuPdXXvovg7KxLcQerKT+FbKbq8DySJX2rnOA77k\nUG4c9BGf/L1uBkAT8dpHLk6Uf5BfmypxUkydSWT1xfTDnw1MqxO0MsLlAHOR3J7c\noW9kLcOLuCQn1hBEwfZv7VSWBkGXSmKfp0LLIxAFgRtv+Dh+rcMMRdJgKr1V3FU+\nrZ1+ZAfYiBpQJFPjv70vx+rGEgS801D3PJxBZUEy4Ic4ZYaKNhK9x9PRQuWcIBuW\n6eTe/6lKWZeyxCumLLdiS75mF2oTcBaWeoc3QxrPRV15eDKeYJMbhnUai/7lSrhs\nEWCkKR1RivgF4slYmtNE5ZPGZ/d61zjwn2xi4xNJVs8q9WRPMpHp0vCyMwARAQAB\ntDFGZWRvcmEgKDMzKSA8ZmVkb3JhLTMzLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJeMAb7AhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRBJ/XdJlXD/MZm2D/9kriL43vd3+0DNMeA82n2v9mSR2PQqKny39xNlYPyy/1yZ\nP/KXoa4NYSCA971LSd7lv4n/h5bEKgGHxZfttfOzOnWMVSSTfjRyM/df/NNzTUEV\n7ORA5GW18g8PEtS7uRxVBf3cLvWu5q+8jmqES5HqTAdGVcuIFQeBXFN8Gy1Jinuz\nAH8rJSdkUeZ0cehWbERq80BWM9dhad5dW+/+Gv0foFBvP15viwhWqajr8V0B8es+\n2/tHI0k86FAujV5i0rrXl5UOoLilO57QQNDZH/qW9GsHwVI+2yecLstpUNLq+EZC\nGqTZCYoxYRpl0gAMbDLztSL/8Bc0tJrCRG3tavJotFYlgUK60XnXlQzRkh9rgsfT\nEXbQifWdQMMogzjCJr0hzJ+V1d0iozdUxB2ZEgTjukOvatkB77DY1FPZRkSFIQs+\nfdcjazDIBLIxwJu5QwvTNW8lOLnJ46g4sf1WJoUdNTbR0BaC7HHj1inVWi0p7IuN\n66EPGzJOSjLK+vW+J0ncPDEgLCV74RF/0nR5fVTdrmiopPrzFuguHf9S9gYI3Zun\nYl8FJUu4kRO6JPPTicUXWX+8XZmE94aK14RCJL23nOSi8T1eW8JLW43dCBRO8QUE\nAso1t2pypm/1zZexJdOV8yGME3g5l2W6PLgpz58DBECgqc/kda+VWgEAp7rO2A==\n=EPL3\n-----END PGP PUBLIC KEY BLOCK-----\n",
      "check_gpg": false,
      "rhsm": true
    },
    {
      "baseurl": "https://localhost:8009/repo",
      "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBF4wBvsBEADQmcGbVUbDRUoXADReRmOOEMeydHghtKC9uRs9YNpGYZIB+bie\nbGYZmflQayfh/wEpO2W/IZfGpHPL42V7SbyvqMjwNls/fnXsCtf4LRofNK8Qd9fN\nkYargc9R7BEz/mwXKMiRQVx+DzkmqGWy2gq4iD0/mCyf5FdJCE40fOWoIGJXaOI1\nTz1vWqKwLS5T0dfmi9U4Tp/XsKOZGvN8oi5h0KmqFk7LEZr1MXarhi2Va86sgxsF\nQcZEKfu5tgD0r00vXzikoSjn3qA5JW5FW07F1pGP4bF5f9J3CZbQyOjTSWMmmfTm\n2d2BURWzaDiJN9twY2yjzkoOMuPdXXvovg7KxLcQerKT+FbKbq8DySJX2rnOA77k\nUG4c9BGf/L1uBkAT8dpHLk6Uf5BfmypxUkydSWT1xfTDnw1MqxO0MsLlAHOR3J7c\noW9kLcOLuCQn1hBEwfZv7VSWBkGXSmKfp0LLIxAFgRtv+Dh+rcMMRdJgKr1V3FU+\nrZ1+ZAfYiBpQJFPjv70vx+rGEgS801D3PJxBZUEy4Ic4ZYaKNhK9x9PRQuWcIBuW\n6eTe/6lKWZeyxCumLLdiS75mF2oTcBaWeoc3QxrPRV15eDKeYJMbhnUai/7lSrhs\nEWCkKR1RivgF4slYmtNE5ZPGZ/d61zjwn2xi4xNJVs8q9WRPMpHp0vCyMwARAQAB\ntDFGZWRvcmEgKDMzKSA8ZmVkb3JhLTMzLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJeMAb7AhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRBJ/XdJlXD/MZm2D/9kriL43vd3+0DNMeA82n2v9mSR2PQqKny39xNlYPyy/1yZ\nP/KXoa4NYSCA971LSd7lv4n/h5bEKgGHxZfttfOzOnWMVSSTfjRyM/df/NNzTUEV\n7ORA5GW18g8PEtS7uRxVBf3cLvWu5q+8jmqES5HqTAdGVcuIFQeBXFN8Gy1Jinuz\nAH8rJSdkUeZ0cehWbERq80BWM9dhad5dW+/+Gv0foFBvP15viwhWqajr8V0B8es+\n2/tHI0k86FAujV5i0rrXl5UOoLilO57QQNDZH/qW9GsHwVI+2yecLstpUNLq+EZC\nGqTZCYoxYRpl0gAMbDLztSL/8Bc0tJrCRG3tavJotFYlgUK60XnXlQzRkh9rgsfT\nEXbQifWdQMMogzjCJr0hzJ+V1d0iozdUxB2ZEgTjukOvatkB77DY1FPZRkSFIQs+\nfdcjazDIBLIxwJu5QwvTNW8lOLnJ46g4sf1WJoUdNTbR0BaC7HHj1inVWi0p7IuN\n66EPGzJOSjLK+vW+J0ncPDEgLCV74RF/0nR5fVTdrmiopPrzFuguHf9S9gYI3Zun\nYl8FJUu4kRO6JPPTicUXWX+8XZmE94aK14RCJL23nOSi8T1eW8JLW43dCBRO8QUE\nAso1t2pypm/1zZexJdOV8yGME3g5l2W6PLgpz58DBECgqc/kda+VWgEAp7rO2A==\n=EPL3\n-----END PGP PUBLIC KEY BLOCK-----\n",
      "check_gpg": false,
      "rhsm": true
    }
  ],
  "aarch64": [
    {
      "baseurl": "https://localhost:8008/repo",
      "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBF4wBvsBEADQmcGbVUbDRUoXADReRmOOEMeydHghtKC9uRs9YNpGYZIB+bie\nbGYZmflQayfh/wEpO2W/IZfGpHPL42V7SbyvqMjwNls/fnXsCtf4LRofNK8Qd9fN\nkYargc9R7BEz/mwXKMiRQVx+DzkmqGWy2gq4iD0/mCyf5FdJCE40fOWoIGJXaOI1\nTz1vWqKwLS5T0dfmi9U4Tp/XsKOZGvN8oi5h0KmqFk7LEZr1MXarhi2Va86sgxsF\nQcZEKfu5tgD0r00vXzikoSjn3qA5JW5FW07F1pGP4bF5f9J3CZbQyOjTSWMmmfTm\n2d2BURWzaDiJN9twY2yjzkoOMuPdXXvovg7KxLcQerKT+FbKbq8DySJX2rnOA77k\nUG4c9BGf/L1uBkAT8dpHLk6Uf5BfmypxUkydSWT1xfTDnw1MqxO0MsLlAHOR3J7c\noW9kLcOLuCQn1hBEwfZv7VSWBkGXSmKfp0LLIxAFgRtv+Dh+rcMMRdJgKr1V3FU+\nrZ1+ZAfYiBpQJFPjv70vx+rGEgS801D3PJxBZUEy4Ic4ZYaKNhK9x9PRQuWcIBuW\n6eTe/6lKWZeyxCumLLdiS75mF2oTcBaWeoc3QxrPRV15eDKeYJMbhnUai/7lSrhs\nEWCkKR1RivgF4slYmtNE5ZPGZ/d61zjwn2xi4xNJVs8q9WRPMpHp0vCyMwARAQAB\ntDFGZWRvcmEgKDMzKSA8ZmVkb3JhLTMzLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJeMAb7AhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRBJ/XdJlXD/MZm2D/9kriL43vd3+0DNMeA82n2v9mSR2PQqKny39xNlYPyy/1yZ\nP/KXoa4NYSCA971LSd7lv4n/h5bEKgGHxZfttfOzOnWMVSSTfjRyM/df/NNzTUEV\n7ORA5GW18g8PEtS7uRxVBf3cLvWu5q+8jmqES5HqTAdGVcuIFQeBXFN8Gy1Jinuz\nAH8rJSdkUeZ0cehWbERq80BWM9dhad5dW+/+Gv0foFBvP15viwhWqajr8V0B8es+\n2/tHI0k86FAujV5i0rrXl5UOoLilO57QQNDZH/qW9GsHwVI+2yecLstpUNLq+EZC\nGqTZCYoxYRpl0gAMbDLztSL/8Bc0tJrCRG3tavJotFYlgUK60XnXlQzRkh9rgsfT\nEXbQifWdQMMogzjCJr0hzJ+V1d0iozdUxB2ZEgTjukOvatkB77DY1FPZRkSFIQs+\nfdcjazDIBLIxwJu5QwvTNW8lOLnJ46g4sf1WJoUdNTbR0BaC7HHj1inVWi0p7IuN\n66EPGzJOSjLK+vW+J0ncPDEgLCV74RF/0nR5fVTdrmiopPrzFuguHf9S9gYI3Zun\nYl8FJUu4kRO6JPPTicUXWX+8XZmE94aK14RCJL23nOSi8T1eW8JLW43dCBRO8QUE\nAso1t2pypm/1zZexJdOV8yGME3g5l2W6PLgpz58DBECgqc/kda+VWgEAp7rO2A==\n=EPL3\n-----END PGP PUBLIC KEY BLOCK-----\n",
      "check_gpg": false,
      "rhsm": true
    },
    {
      "baseurl": "https://localhost:8009/repo",
      "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBF4wBvsBEADQmcGbVUbDRUoXADReRmOOEMeydHghtKC9uRs9YNpGYZIB+bie\nbGYZmflQayfh/wEpO2W/IZfGpHPL42V7SbyvqMjwNls/fnXsCtf4LRofNK8Qd9fN\nkYargc9R7BEz/mwXKMiRQVx+DzkmqGWy2gq4iD0/mCyf5FdJCE40fOWoIGJXaOI1\nTz1vWqKwLS5T0dfmi9U4Tp/XsKOZGvN8oi5h0KmqFk7LEZr1MXarhi2Va86sgxsF\nQcZEKfu5tgD0r00vXzikoSjn3qA5JW5FW07F1pGP4bF5f9J3CZbQyOjTSWMmmfTm\n2d2BURWzaDiJN9twY2yjzkoOMuPdXXvovg7KxLcQerKT+FbKbq8DySJX2rnOA77k\nUG4c9BGf/L1uBkAT8dpHLk6Uf5BfmypxUkydSWT1xfTDnw1MqxO0MsLlAHOR3J7c\noW9kLcOLuCQn1hBEwfZv7VSWBkGXSmKfp0LLIxAFgRtv+Dh+rcMMRdJgKr1V3FU+\nrZ1+ZAfYiBpQJFPjv70vx+rGEgS801D3PJxBZUEy4Ic4ZYaKNhK9x9PRQuWcIBuW\n6eTe/6lKWZeyxCumLLdiS75mF2oTcBaWeoc3QxrPRV15eDKeYJMbhnUai/7lSrhs\nEWCkKR1RivgF4slYmtNE5ZPGZ/d61zjwn2xi4xNJVs8q9WRPMpHp0vCyMwARAQAB\ntDFGZWRvcmEgKDMzKSA8ZmVkb3JhLTMzLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v\ncmc+iQI4BBMBAgAiBQJeMAb7AhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK\nCRBJ/XdJlXD/MZm2D/9kriL43vd3+0DNMeA82n2v9mSR2PQqKny39xNlYPyy/1yZ\nP/KXoa4NYSCA971LSd7lv4n/h5bEKgGHxZfttfOzOnWMVSSTfjRyM/df/NNzTUEV\n7ORA5GW18g8PEtS7uRxVBf3cLvWu5q+8jmqES5HqTAdGVcuIFQeBXFN8Gy1Jinuz\nAH8rJSdkUeZ0cehWbERq80BWM9dhad5dW+/+Gv0foFBvP15viwhWqajr8V0B8es+\n2/tHI0k86FAujV5i0rrXl5UOoLilO57QQNDZH/qW9GsHwVI+2yecLstpUNLq+EZC\nGqTZCYoxYRpl0gAMbDLztSL/8Bc0tJrCRG3tavJotFYlgUK60XnXlQzRkh9rgsfT\nEXbQifWdQMMogzjCJr0hzJ+V1d0iozdUxB2ZEgTjukOvatkB77DY1FPZRkSFIQs+\nfdcjazDIBLIxwJu5QwvTNW8lOLnJ46g4sf1WJoUdNTbR0BaC7HHj1inVWi0p7IuN\n66EPGzJOSjLK+vW+J0ncPDEgLCV74RF/0nR5fVTdrmiopPrzFuguHf9S9gYI3Zun\nYl8FJUu4kRO6JPPTicUXWX+8XZmE94aK14RCJL23nOSi8T1eW8JLW43dCBRO8QUE\nAso1t2pypm/1zZexJdOV8yGME3g5l2W6PLgpz58DBECgqc/kda+VWgEAp7rO2A==\n=EPL3\n-----END PGP PUBLIC KEY BLOCK-----\n",
      "check_gpg": false,
      "rhsm": true
    }
  ]
}
STOPHERE

sudo systemctl restart osbuild-composer
sudo composer-cli status show

BLUEPRINT_FILE=/tmp/bp.toml
BLUEPRINT_NAME=fishy

cat > "$BLUEPRINT_FILE" << STOPHERE
name = "${BLUEPRINT_NAME}"
description = "A base system with fish"
version = "0.0.1"

[[packages]]
name = "fish"
STOPHERE

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
  sudo journalctl -xe --unit osbuild-worker
  exit 1
fi
COMPOSE_ID=$(jq -r '.build_id' "${COMPOSE_START}")

# Wait for the compose to finish.
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "${COMPOSE_INFO}" > /dev/null
    COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30

done

sudo journalctl -xe --unit osbuild-composer
sudo journalctl -xe --unit osbuild-worker

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

greenprint "Putting things back to their previous configuration"
sudo rm -f /etc/yum.repos.d/osbuild-override.repo
sudo rm -f /etc/yum.repos.d/redhat.repo
sudo mv /etc/yum.repos.d/redhat.repo.backup /etc/yum.repos.d/redhat.repo || echo "no redhat.repo"
sudo mv /etc/osbuild-composer/repositories/fedora-33.json.backup /etc/osbuild-composer/repositories/fedora-33.json
sudo rm -rf /etc/httpd/conf.d
sudo mv /etc/httpd/conf.d.backup /etc/httpd/conf.d
