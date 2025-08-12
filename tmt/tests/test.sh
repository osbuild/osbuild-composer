#!/bin/bash
set -euox pipefail

cd ../../ || exit 1

schutzbot/deploy.sh

function run_tests() {
	if [ "$TEST_CASE" = "edge-commit" ]; then
		/usr/libexec/tests/osbuild-composer/ostree.sh
	elif [ "$TEST_CASE" = "edge-installer" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-ng.sh
	elif [ "$TEST_CASE" = "edge-installer-fips" ]; then
		FIPS=true /usr/libexec/tests/osbuild-composer/ostree-ng.sh
	elif [ "$TEST_CASE" = "edge-raw-image" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-raw-image.sh
	elif [ "$TEST_CASE" = "edge-simplified-installer" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-simplified-installer.sh
	elif [ "$TEST_CASE" = "edge-ignition" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-ignition.sh
	elif [ "$TEST_CASE" = "edge-minimal" ]; then
		/usr/libexec/tests/osbuild-composer/minimal-raw.sh
	elif [ "$TEST_CASE" = "edge-ami-image" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-ami-image.sh
	elif [ "$TEST_CASE" = "edge-ami-image-fips" ]; then
		FIPS=true /usr/libexec/tests/osbuild-composer/ostree-ami-image.sh
	elif [ "$TEST_CASE" = "edge-vsphere" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-vsphere.sh
	elif [ "$TEST_CASE" = "edge-qcow2" ]; then
		/usr/libexec/tests/osbuild-composer/ostree-iot-qcow2.sh
	else
		echo "Error: Test case $TEST_CASE not found!"
		exit 1
	fi
}

run_tests
exit 0