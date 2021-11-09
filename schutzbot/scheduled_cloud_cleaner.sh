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

az login --service-principal --username "${V2_AZURE_CLIENT_ID}" --password "${V2_AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"

# List all resources from AZURE_RESOURCE_GROUP
RESOURCE_LIST=$(az resource list -g "$AZURE_RESOURCE_GROUP")
RESOURCE_COUNT=$( echo "$RESOURCE_LIST" | jq .[].name | wc -l)

# filter out resources older than X hours
HOURS_BACK="${HOURS_BACK:-6}"
DELETE_TIME=$(date -d "- $HOURS_BACK hours" +%s)
OLD_RESOURCE_LIST_NAMES=()
for i in $(seq 0 $(("$RESOURCE_COUNT"-1))); do
    RESOURCE_TIME=$(echo "$RESOURCE_LIST" | jq ".[$i].createdTime" | tr -d '"')
    RESOURCE_TYPE=$(echo "$RESOURCE_LIST" | jq ".[$i].type" | tr -d '"')
    RESOURCE_TIME_SECONDS=$(date -d "$RESOURCE_TIME" +%s)
    if [[ "$RESOURCE_TIME_SECONDS" -lt "$DELETE_TIME" && "$RESOURCE_TYPE" != Microsoft.Storage/storageAccounts ]]; then
        OLD_RESOURCE_LIST_NAMES+=("$(echo "$RESOURCE_LIST" | jq .["$i"].name | sed -e 's/^[^-]*-//' | tr -d '"')")
    fi
done

#Exit early if no there are no resources to delete
if [ ${#OLD_RESOURCE_LIST_NAMES[@]} == 0 ]; then
    echo "Nothing to delete in the standard storage account."
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

# Explicitly check the other storage accounts (mostly the api test one)
STORAGE_ACCOUNT_LIST=$(az resource list -g "$AZURE_RESOURCE_GROUP" --resource-type Microsoft.Storage/storageAccounts)
STORAGE_ACCOUNT_COUNT=$(echo "$STORAGE_ACCOUNT_LIST" | jq .[].name | wc -l)
DELETE_TIME=$(date -d "- $HOURS_BACK hours" +%s)
for i in $(seq 0 $(("$STORAGE_ACCOUNT_COUNT"-1))); do
    STORAGE_ACCOUNT_NAME=$(echo "$STORAGE_ACCOUNT_LIST" | jq .["$i"].name | tr -d '"')
    if [ "$AZURE_STORAGE_ACCOUNT" = "$STORAGE_ACCOUNT_NAME" ]; then
        echo "Not checking default storage account $AZURE_STORAGE_ACCOUNT in other storage account script."
        continue
    fi

    echo "Checking storage account $STORAGE_ACCOUNT_NAME for old blobs."
    CONTAINER_LIST=$(az storage container list --account-name "$STORAGE_ACCOUNT_NAME")
    CONTAINER_COUNT=$(echo "$CONTAINER_LIST" | jq .[].name | wc -l)
    for i2 in $(seq 0 $(("$CONTAINER_COUNT"-1))); do
        CONTAINER_NAME=$(echo "$CONTAINER_LIST" | jq .["$i2"].name | tr -d '"')
        BLOB_LIST=$(az storage blob list --account-name "$STORAGE_ACCOUNT_NAME" --container-name "$CONTAINER_NAME")
        BLOB_COUNT=$(echo "$BLOB_LIST" | jq .[].name | wc -l)
        for i3 in $(seq 0 $(("$BLOB_COUNT"-1))); do
            BLOB_NAME=$(echo "$BLOB_LIST" | jq .["$i3"].name | tr -d '"')
            BLOB_TIME=$(echo "$BLOB_LIST" | jq .["$i3"].properties.lastModified | tr -d '"')
            BLOB_TIME_SECONDS=$(date -d "$BLOB_TIME" +%s)
            if [[ "$BLOB_TIME_SECONDS" -lt "$DELETE_TIME" ]]; then
                echo "Deleting blob $BLOB_NAME in $STORAGE_ACCOUNT_NAME's $CONTAINER_NAME container."
                az storage blob delete --only-show-errors --account-name "$STORAGE_ACCOUNT_NAME" --container-name "$CONTAINER_NAME" -n "$BLOB_NAME"
            fi
        done
    done
done

echo "Azure cleanup complete!"
