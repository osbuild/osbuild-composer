#!/bin/bash
set -eu

SHARE_DIR=/tmp/osbuild-composer-koji-test

koji_stop () {
  echo "Shutting down containers, please wait..."

  ${CONTAINER_RUNTIME} stop org.osbuild.koji.koji || true
  ${CONTAINER_RUNTIME} rm org.osbuild.koji.koji || true

  ${CONTAINER_RUNTIME} stop org.osbuild.koji.kdc || true
  ${CONTAINER_RUNTIME} rm org.osbuild.koji.kdc || true

  ${CONTAINER_RUNTIME} stop org.osbuild.koji.postgres || true
  ${CONTAINER_RUNTIME} rm org.osbuild.koji.postgres || true

  ${CONTAINER_RUNTIME} network rm -f org.osbuild.koji || true

  rm -rf "${SHARE_DIR}" || true
}

koji_clean_up_bad_start ()  {
  # remember the exit code, so we can report it later
  EXIT_CODE=$?
  echo "Start failed, removing containers."

  koji_stop

  exit $EXIT_CODE
}


# helper to simplify sql queries to the postgres instance
psql_cmd () {
  ${CONTAINER_RUNTIME} exec org.osbuild.koji.postgres psql -U koji -d koji "$@"
}

# helper to simplify running commands in the kdc container
kdc_exec() {
  ${CONTAINER_RUNTIME} exec org.osbuild.koji.kdc "$@"
}

koji_start() {
  trap koji_clean_up_bad_start EXIT

  # create a share directory which is used to share files between the host and containers
  mkdir "${SHARE_DIR}"

  # generate self-signed certificates in the share directory
  openssl req -new -nodes -x509 -days 365 -keyout "${SHARE_DIR}/ca-key.pem" -out "${SHARE_DIR}/ca-crt.pem" -subj "/CN=osbuild.org"
  openssl genrsa -out "${SHARE_DIR}/key.pem" 2048
  openssl req -new -sha256 -key "${SHARE_DIR}/key.pem"	-out "${SHARE_DIR}/csr.pem" -subj "/CN=localhost"
  openssl x509 -req -in "${SHARE_DIR}/csr.pem"  -CA "${SHARE_DIR}/ca-crt.pem" -CAkey "${SHARE_DIR}/ca-key.pem" -CAcreateserial -out "${SHARE_DIR}/crt.pem"

  ${CONTAINER_RUNTIME} network create org.osbuild.koji

  ${CONTAINER_RUNTIME} run -d --name org.osbuild.koji.postgres --network org.osbuild.koji \
    -e POSTGRES_USER=koji \
    -e POSTGRES_PASSWORD=kojipass \
    -e POSTGRES_DB=koji \
    docker.io/library/postgres:12-alpine

  ${CONTAINER_RUNTIME} run -d --name org.osbuild.koji.kdc \
    --network org.osbuild.koji \
    -v "${SHARE_DIR}:/share:z" \
    -p 88:88/udp \
    quay.io/osbuild/kdc:v1

  # initialize krb pricipals and create keytabs for them
  # HTTP/localhost@LOCAL for kojihub
  kdc_exec kadmin.local -r LOCAL add_principal -randkey HTTP/localhost@LOCAL
  kdc_exec kadmin.local -r LOCAL ktadd -k /share/koji.keytab HTTP/localhost@LOCAL
  kdc_exec chmod 644 /share/koji.keytab

  # osbuild-krb@LOCAL for koji clients
  kdc_exec kadmin.local -r LOCAL add_principal -randkey osbuild-krb@LOCAL
  kdc_exec kadmin.local -r LOCAL ktadd -k /share/client.keytab osbuild-krb@LOCAL
  kdc_exec chmod 644 /share/client.keytab

  ${CONTAINER_RUNTIME} run -d --name org.osbuild.koji.koji --network org.osbuild.koji \
    -v "${SHARE_DIR}:/share:z" \
    -p 80:80 \
    -p 443:443 \
    -e POSTGRES_USER=koji \
    -e POSTGRES_PASSWORD=kojipass \
    -e POSTGRES_DB=koji \
    -e POSTGRES_HOST=org.osbuild.koji.postgres \
    quay.io/osbuild/koji:v1

  # TODO: we need to wait for the database to be initialized here. A better method should be used.
  sleep 2

  # create koji users
  # kojiadmin/kojipass    - admin
  # osbuild/osbuildpass   - regular user
  # osbuild-krb:          - regular user authenticated with Kerberos principal osbuild-krb@LOCAL
  psql_cmd -c "insert into users (name, password, status, usertype) values ('kojiadmin', 'kojipass', 0, 0)" >/dev/null
  psql_cmd -c "insert into user_perms (user_id, perm_id, creator_id) values (1, 1, 1)" >/dev/null
  psql_cmd -c "insert into users (name, password, status, usertype) values ('osbuild', 'osbuildpass', 0, 0)" >/dev/null
  psql_cmd -c "insert into users (name, status, usertype) values ('osbuild-krb', 0, 0)" >/dev/null
  psql_cmd -c "insert into user_krb_principals (user_id, krb_principal) values (3, 'osbuild-krb@LOCAL')" >/dev/null

  # create content generator osbuild, give osbuild and osbuild-krb users access to it
  psql_cmd -c "insert into content_generator (name) values ('osbuild')" >/dev/null
  psql_cmd -c "insert into cg_users (cg_id, user_id, creator_id, active) values (1, 2, 1, true), (1, 3, 1, true)" >/dev/null

  echo "Containers are running, to stop them use:"
  echo "$0 stop"

  trap - EXIT
}

# check arguments
if [[ $# -ne 1 || ( "$1" != "start" && "$1" != "stop" ) ]]; then
  cat <<DOC
usage: $0 start|stop

start - starts the koji containers
stop  - stops and removes the koji containers
DOC
  exit 3
fi

# this script must be run as root
if [ $UID != 0 ]; then
  echo This script must be run as root.
  exit 1
fi

# decide whether podman or docker should be used
if which podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

if [ $1 == "start" ]; then
  koji_start
fi

if [ $1 == "stop" ]; then
  koji_stop
fi
