#!/bin/bash
set -euo pipefail

#
# Script that executes different high level sanity tests writen in GO. 
#

WORKING_DIRECTORY=/usr/libexec/osbuild-composer
TESTS_PATH=/usr/libexec/osbuild-composer-test
mkdir --parents /tmp/logs
LOGS_DIRECTORY=$(mktemp --directory --tmpdir=/tmp/logs)

PASSED_TESTS=()
FAILED_TESTS=()

TEST_CASES=(
  "osbuild-weldr-tests"
  "osbuild-dnf-json-tests"
  "osbuild-composer-cli-tests"
  "osbuild-auth-tests"
  "osbuild-composer-dbjobqueue-tests"
)

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

# Run a test case and store the result as passed or failed.
run_test_case () {
    TEST_NAME=$(basename "$1")
    echo
    test_divider
    echo "🏃🏻 Running test: ${TEST_NAME}"
    test_divider

    if sudo "${1}" -test.v | tee "${LOGS_DIRECTORY}"/"${TEST_NAME}".log; then
        PASSED_TESTS+=("$TEST_NAME")
    else
        FAILED_TESTS+=("$TEST_NAME")
    fi

    test_divider
    echo
}


# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Set up a basic postgres db
sudo dnf install -y go postgresql postgresql-server postgresql-contrib
PWFILE=$(sudo -u postgres mktemp)
cat <<EOF | sudo -u postgres tee "$PWFILE"
foobar
EOF
PGSETUP_INITDB_OPTIONS="-A password -U postgres --pwfile=$PWFILE" \
    sudo -E postgresql-setup --initdb --unit postgresql
sudo systemctl start postgresql
PGUSER=postgres PGPASSWORD=foobar PGHOST=localhost PGPORT=5432 \
    sudo -E -u postgres psql -c "CREATE DATABASE osbuildcomposer;"

# Initialize a module in a temp dir so we can get tern without introducing
# vendoring inconsistency
pushd "$(mktemp -d)"
go mod init temp
go get github.com/jackc/tern
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
      go run github.com/jackc/tern migrate -m /usr/share/tests/osbuild-composer/schemas
popd

# Change to the working directory.
cd $WORKING_DIRECTORY

# Run each test case.
for TEST_CASE in "${TEST_CASES[@]}"; do
    run_test_case ${TESTS_PATH}/"$TEST_CASE"
done

# Print a report of the test results.
test_divider
echo "😃 Passed tests:" "${PASSED_TESTS[@]}"
echo "☹ Failed tests:" "${FAILED_TESTS[@]}"
test_divider

# Exit with a failure if tests were executed and any of them failed.
if [ ${#PASSED_TESTS[@]} -gt 0 ] && [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "🎉 All tests passed."
    exit 0
else
    echo "🔥 One or more tests failed."
    exit 1
fi
