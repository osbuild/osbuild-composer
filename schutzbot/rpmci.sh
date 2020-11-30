#!/bin/bash

set -ex

sudo dnf install git jq -y

# Use a static one to speed up development
GIT_COMMIT="207080024408be5698669058ef49d265fbd723b6"

cat > /tmp/rpmci-config.json << STOPHERE
{
	"target": {
		"virtualization": {
			"type": "ec2",
			"ec2": {
				"image_id": "ami-091ea242bbc6da43d",
				"access_key_id": "${AWS_ACCESS_KEY_ID}",
				"secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
				"region_name": "eu-central-1"

			}
		},
		"rpm": "osbuild-composer-tests"
	},
	"rpm_repo": {
		"provider": "existing_url",
		"existing_url": {
			"baseurl": "http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/fedora-33/x86_64/${GIT_COMMIT}/"
		}
	},
	"test_invocation": {
		"machine": "target",
		"invoke": [
			"/usr/libexec/tests/osbuild-composer/api.sh",
			"/usr/libexec/tests/osbuild-composer/aws.sh",
			"/usr/libexec/tests/osbuild-composer/base_tests.sh",
			"/usr/libexec/tests/osbuild-composer/image_tests.sh",
			"/usr/libexec/tests/osbuild-composer/koji.sh",
			"/usr/libexec/tests/osbuild-composer/ostree.sh",
			"/usr/libexec/tests/osbuild-composer/qemu.sh"
		]
	}
}
STOPHERE

cat /tmp/rpmci-config.json | jq .

pushd /tmp || exit
git clone https://github.com/osbuild/rpmci.git
pushd rpmci || exit
python3 -m venv venv
source venv/bin/activate
pip install .
mkdir -p /tmp/cache
python -m rpmci --cache /tmp/cache run < /tmp/rpmci-config.json
