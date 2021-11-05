#!/bin/bash

# use tee, otherwise shellcheck complains
sudo journalctl --boot | tee journal-log >/dev/null

# As it might contain sensitive information and is important for debugging
# purposes, encrypt journal-log with a symmetric passphrase.
gpg --batch --yes --passphrase "$GPG_SYMMETRIC_PASSPHRASE" -o journal-log.gpg --symmetric journal-log
rm journal-log
