#!/bin/sh
# /usr/local/sbin/temp-disk-dataloss-warning
# Write dataloss warning file on mounted Azure resource disk

AZURE_RESOURCE_DISK_PART1="/dev/disk/cloud/azure_resource-part1"

MOUNTPATH=$(grep "$AZURE_RESOURCE_DISK_PART1" /etc/fstab | tr '\t' ' ' | cut -d' ' -f2)
if [ -z "$MOUNTPATH" ]; then
    echo "There is no mountpoint of $AZURE_RESOURCE_DISK_PART1 in /etc/fstab"
    exit 0
fi

if [ "$MOUNTPATH" = "none" ]; then
    echo "Mountpoint of $AZURE_RESOURCE_DISK_PART1 is not a path"
    exit 1
fi

if ! mountpoint -q "$MOUNTPATH"; then
    echo "$AZURE_RESOURCE_DISK_PART1 is not mounted at $MOUNTPATH"
    exit 1
fi

echo "Creating a dataloss warning file at ${MOUNTPATH}/DATALOSS_WARNING_README.txt"

cat <<'EOF' > "${MOUNTPATH}/DATALOSS_WARNING_README.txt"
WARNING: THIS IS A TEMPORARY DISK.

Any data stored on this drive is SUBJECT TO LOSS and THERE IS NO WAY TO RECOVER IT.

Please do not use this disk for storing any personal or application data.

EOF
