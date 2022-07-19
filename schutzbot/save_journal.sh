#!/bin/bash

# use tee, otherwise shellcheck complains
sudo journalctl --boot | tee journal-log >/dev/null

# copy journal to artifacts folder which is then uploaded to secure S3 location
cp journal-log "${ARTIFACTS:-/tmp/artifacts}"
