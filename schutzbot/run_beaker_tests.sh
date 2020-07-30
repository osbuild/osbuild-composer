#!/bin/bash
# Prepare to run tests by submitting jobs to Beaker.
set -euxo pipefail

# Install requirements.
sudo dnf -y install beaker-client krb5-workstation

# Set up basic Beaker client configuration.
mkdir -p ~/.beaker_client
tee ~/.beaker_client/config > /dev/null << EOF
HUB_URL = "https://beaker.engineering.redhat.com"
AUTH_METHOD = "krbv"
KRB_REALM = "REDHAT.COM"
EOF

# Get a kerberos ticket.
PRINCIPAL=$(klist -k $BEAKER_KEYTAB | tail -n 1 | awk '{print $2}' | sed 's/IPA\.//')
kinit -k -t /tmp/beaker.keytab $PRINCIPAL

# Verify that we can talk to Beaker's API.
bkr whoami --insecure