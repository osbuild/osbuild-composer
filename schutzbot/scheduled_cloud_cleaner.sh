#!/bin/bash

# Azure cleanup
if ! hash az; then
    # this installation method is taken from the official docs:
    # https://docs.microsoft.com/cs-cz/cli/azure/install-azure-cli-linux?pivots=dnf
    sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
    echo -e "[azure-cli]
name=Azure CLI
baseurl=https://packages.microsoft.com/yumrepos/azure-cli
enabled=1
gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc" | sudo tee /etc/yum.repos.d/azure-cli.repo

  greenprint "Installing azure-cli"
  sudo dnf install -y azure-cli
  az version
fi

az login --service-principal --username "${AZURE_CLIENT_ID}" --password "${AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"

# List all resources from AZURE_RESOURCE_GROUP
RESOURCE_LIST=$(az resource list -g "$AZURE_RESOURCE_GROUP")
RESOURCE_COUNT=$( echo "$RESOURCE_LIST" | jq .[].name | wc -l)

# filter out resources older than X hours
DELETE_TIME=$(date -d "- $HOURS_BACK hours" +%s)
OLD_RESOURCE_LIST_NAMES=()
for i in $(seq 0 $(("$RESOURCE_COUNT"-1))); do
    RESOURCE_TIME=$(echo "$RESOURCE_LIST" | jq .[$i].createdTime | tr -d '"')
    RESOURCE_TYPE=$(echo "$RESOURCE_LIST" | jq .[$i].type | tr -d '"')
    RESOURCE_TIME_SECONDS=$(date -d "$RESOURCE_TIME" +%s)
    if [[ "$RESOURCE_TIME_SECONDS" -lt "$DELETE_TIME" && "$RESOURCE_TYPE" != Microsoft.Storage/storageAccounts ]]; then
        OLD_RESOURCE_LIST_NAMES+=("$(echo "$RESOURCE_LIST" | jq .["$i"].name | sed -e 's/^[^-]*-//' | tr -d '"')")
    fi
done

#Exit early if no there are no resources to delete
if [ ${#OLD_RESOURCE_LIST_NAMES[@]} == 0 ]; then
    echo "Nothing to delete."
    exit 0
fi

# Keep only unique resource names
mapfile -t RESOURCE_TO_DELETE_LIST  < <(printf "%s\n" "${OLD_RESOURCE_LIST_NAMES[@]}" | sort -u)
echo "${RESOURCE_TO_DELETE_LIST[@]}"

TO_DELETE_COUNT=${#RESOURCE_TO_DELETE_LIST[@]}
echo "There are resources from $TO_DELETE_COUNT test runs to delete."

for i in $(seq 0 $(("$TO_DELETE_COUNT"-1))); do
    echo "Running cloud-cleaner in Azure for resources with TEST_ID: ${RESOURCE_TO_DELETE_LIST[$i]}"
    TEST_ID=${RESOURCE_TO_DELETE_LIST[$i]} /usr/libexec/osbuild-composer-test/cloud-cleaner
done

echo "Azure cleanup complete!"
