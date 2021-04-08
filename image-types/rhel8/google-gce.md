# Google Compute Engine (GCE) RHEL Guest Image

The GCE Image is meant to be run in the Google Compute Engine environment.

*GCE Images are an immutable representation of a persistent disk. They are used as a template when creating a persistent disk of an instance.*

## Importing virtual disks to GCE

In general, Google supports two ways of importing virtual disks into GCE to create images from them:

1. **Importing of virtual disks** of various types using [*image_import* daisy workflow][daisy-wf-image_import]. It is the *default* approach to use for importing custom images to GCE. This approach uses Google Cloud Build API to run the workflow.
2. **Manual import of virtual disks**. This approach imports the virtual disk "as is" using the Google Compute Engine [`images.insert` API method][compute-images-insert].

[daisy-wf-image_import]: https://github.com/GoogleCloudPlatform/compute-image-tools/tree/master/daisy_workflows/image_import
[compute-images-insert]: https://cloud.google.com/compute/docs/reference/rest/v1/images/insert

### Importing virtual disks (using *Daisy* workflow)

Google supports [importing virtual disks][gce-import-virtual-disks] of various formats using their [*image_import* daisy workflow][daisy-wf-image_import]. Supported formats include *raw*, *qcow2*, *qcow*, *vmdk*, *vdi*, *vhd*, *vhdx*, *qed*, *vpc*.

The basic high-level description of what the workflow does is as follows:

1. Create a new empty disk for the resulting GCE Image. Its size is the size of the imported image, aligned to whole GBs + 1 GB, but always at least 10 GB.
2. Copy the content of the image uploaded to GCP Storage Bucket to the empty disk using `qemu-img convert "${IMAGE_PATH}" -p -O raw -S 512b ${DISK_DEV}`.
3. If the image is marked as *bootable*, which is the default, then modify it, or "translate" in Google's terms, using a [Python script][image-import-el-translate] run on the imported image using *libguestfs*. For the full list of modifications, check out the [Python script][image-import-el-translate], but in general it installs Google guest tools, disables floppy kernel module, modifies the GRUB configuration, modifies the network configuration, etc.
   * If the user marks the disk as *data disk*, Google interprets this information such as the disk is *not bootable* and it does not modify it. This however does not have any effect on the fact whether the resulting image can be used for a bootable disk. However, the user is then responsible for making sure that the OS image contains all [necessary drivers][gce-os-drivers]. In addition, the OS image should follow [GCE best practices][gce-image-best-practices].
4. Import the disk created by the workflow from the provided virtual disk image as a GCE Image using the Google Compute Engine [`images.insert` API method][compute-images-insert].

**Notes:**

* The workflow allows importing *bootable* images of only [supported operating systems][gce-import-supported-oses]. However, the limitation is imposed mainly by the "translate" step which modifies the imported image. Therefore even images of unsupported operating systems can be imported successfully if marked as *data disk*.
* If this Cloud Build workflow is canceled, no GCE resources created by it are cleaned up and the creator is responsible for cleaning them up. However, there is no easy way to figure out the list of created resources. The only way is to parse the log from the workflow. This has been reported to Google as [issue #182680680][google-issue-182680680].

[gce-import-virtual-disks]: https://cloud.google.com/compute/docs/import/importing-virtual-disks
[image-import-el-translate]: https://github.com/GoogleCloudPlatform/compute-image-tools/blob/master/daisy_workflows/image_import/enterprise_linux/translate.py
[gce-os-drivers]: https://cloud.google.com/compute/docs/images/building-custom-os#hardwaremanifest
[gce-image-best-practices]: https://cloud.google.com/compute/docs/import/configuring-imported-images#optimize_image
[gce-import-supported-oses]: https://cloud.google.com/compute/docs/images/os-details#import
[google-issue-182680680]: https://issuetracker.google.com/issues/182680680

### Manual import of virtual disks

In addition to importing virtual disks using the [*image_import* daisy workflow][daisy-wf-image_import], Google supports also [importing virtual disks manually][gce-manual-import]. In this case, only *raw* images compressed as `.tar.gz` can be imported.

This approach's workflow consists only of the last step (4.) of the workflow for importing virtual images using *Daisy*, described in the previous section.

The user is responsible for making sure, that the imported image conforms to the [requirements][gce-manual-import-req].

The workflow is as follows:

1. Upload the *raw* virtual disk compressed as `.tar.gz` to a GCP Storage Bucket.
2. Use the Google Compute Engine [`images.insert` API method][compute-images-insert] with the `gs://...` path to the uploaded disk as the source.

**Notes:**

* GCP documentation has inconsistent claims, that that rate limiting is applied to the amount of image creation requests which can be done using this workflow. [Specifically, that only one disk image can be created every 10 minutes per project][gce-img-creation-rate-limit]. However other parts of the documentation, specifically for [`gcloud compute images create`][gcloud-compute-image-create] and the Compute Engine [`images.insert` API method][compute-images-insert], which is also used by `gcloud`, don't mention any such drastic rate limiting. Based on the results from testing the API, there seems to be no such limit being applied (30 image creation requests created simultaneously successfully passed without issues). However, there are still general API limits, which apply to all API calls.

[gce-manual-import]: https://cloud.google.com/compute/docs/import/import-existing-image
[gce-manual-import-req]: https://cloud.google.com/compute/docs/import/import-existing-image#requirements
[gce-img-creation-rate-limit]: https://cloud.google.com/compute/docs/images/create-delete-deprecate-private-images#create_image
[gcloud-compute-image-create]: https://cloud.google.com/sdk/gcloud/reference/compute/images/create

### Google's Image Import Precheck Tool

Google provides an [import precheck tool][gce-import-precheck-tool], which can Administrators run on the system, before importing it to CGE, to identify potential compatibility issues.

Checks done by the tool, which are relevant for a Linux distribution are:

* OS check
  * Just checks if the OS falls into the [supported operating systems list][gce-import-supported-oses].
* Disks check
  * Reads mount points.
  * Checks that the root `/` spans exactly one physical device.
  * Checks that other mount points are on the same physical device as the root.
  * The root physical device must use MBR and MBR must use GRUB.
* SSH check
  * Check that SSH daemon is running on `localhost:22`.
  * Checks that the SSH daemon is OpenSSH.

Based on this information, the precheck tool provides very little value for use in *osbuild-composer*'s workflow.

[gce-import-precheck-tool]: https://github.com/GoogleCloudPlatform/compute-image-tools/tree/master/cli_tools/import_precheck/

## Implementation choices (*osbuild-composer* GCE RHEL Guest image)

The RHEL Guest GCE Image is currently built as a *qcow2* image and imported using the [*image_import* daisy workflow][daisy-wf-image_import] as a *data disk*. This is because the initial GCP support has been implemented without the GCE Image type and we reused an existing image type, specifically the Azure VHD type. The reason for importing the image as a *data disk* is that we don't want Google to do any further modifications to the image during the import.

* The GCE Image type has a default size of 4 GB.

* Default `x86_64` images partition table is used.

* The image contains installed Google Guest packages (`google-compute-engine`, `google-osconfig-agent`, `gce-disk-expand`). The services included in these packages are enabled by default and are responsible for initial instance setup (user creation, configuring trusted SSH keys, resizing the disk, etc.). GCE instances can be initialized also using `cloud-init`, however, its integration with GCE is limited, compared to the Google Guest packages. `cloud-init` is not installed on the image.

* The image contains installed Google Cloud SDK.

* `dhcp-client` is not installed, because *NetworkManager* by default uses its internal DHCP client implementation and there is no way to configure *NetworkManager* via an `osbuild` stage at this moment.

* `dnf-automatic` is not installed, because it can not be currently configured via an `osbuild` stage at this moment.

* The image is built as a "Bring Your Own Subscription" (BYOS), meaning that it does not have the Google RHUI client installed. The user is expected to register the system using `subscription-manager`.

* No further customizations done by Google on their GCE RHEL Guest images are currently done.

### Future plans

The long-term plan is to move towards building the image as `.tar.gz` compressed *raw* image and import it using the Compute Engine [`images.insert` API method][compute-images-insert].

## RHEL Guest Images built by Google

Google builds all of the official Guest OS Images using a so-called [Daisy tool][daisy-tool]. *Daisy* allows running multi-step workflows on GCE and among other things to create/delete GCE resources.

The [*image_build* daisy workflow][daisy-wf-image_build-el] defines the actual process to build an Enterprise Linux Guest image. A general high-level documentation of the automated Image creation process with *Daisy* is available in [Google's GitHub repository][daisy-image-creation-doc].

The high-level description of the workflow used to build RHEL Guest Images is as follows:

1. Create a Debian 10 instance with an attached second empty disk. This disk will be the resulting GCE Image.
2. Install a RHEL system using provided installation ISO and generated a kickstart provided to Anaconda. For more details on the actual kickstart's content, check the [section below](#kickstart-files-used-by-google-to-build-RHEL-images). The target of the installation is the attached second empty disk.
3. Create the actual Image from the second disk, once the instance is shut down. The [`images.insert` API method][compute-images-insert], with the second disk as the source, is used for creating the GCE Image.
4. Delete all created resources.

RHEL Guest Images are imported with the following Guest OS features set:
* UEFI_COMPATIBLE
* VIRTIO_SCSI_MULTIQUEUE
* SEV_CAPABLE

[daisy-tool]: https://github.com/GoogleCloudPlatform/compute-image-tools/tree/master/daisy
[daisy-wf-image_build-el]: https://github.com/GoogleCloudPlatform/compute-image-tools/tree/master/daisy_workflows/image_build/enterprise_linux
[daisy-image-creation-doc]: https://github.com/GoogleCloudPlatform/compute-image-tools/blob/master/docs/daisy-automating-image-creation.md

### Implementation choices

The RHEL Guest images available in GCE Marketplace differ from the "standard" RHEL images. Differences are [documented in the GCE documentation][gce-rhel-image-differences].

The image size is 20 GB.

[gce-rhel-image-differences]: https://cloud.google.com/compute/docs/images/os-details#notable-difference-rhel

#### Kernel parameters

`net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d`

#### Partition table

* `/boot/efi`
  * size: 200 MB
  * filesystem type: efi
* `/`
  * size: at least 100 MB, fill available space on the device
  * filesystem type: xfs
  * label: `root`

```shell
# EFI partitioning, creates a GPT partitioned disk.
part /boot/efi --size=200 --fstype=efi --ondrive=sdb
part / --size=100 --grow --ondrive=sdb --label=root --fstype=xfs
```

#### Software management

* A `/etc/yum.repos.d/google-cloud.repo` repo file is created with two repositories:

    ```ini
    [google-compute-engine]
    name=Google Compute Engine
    baseurl=https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable
    enabled=1
    gpgcheck=1
    repo_gpgcheck=0
    gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
        https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
    
    [google-cloud-sdk]
    name=Google Cloud SDK
    baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64
    enabled=1
    gpgcheck=1
    repo_gpgcheck=0
    gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
        https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
    ```

* GPG keys used for Google repositories are imported:

    ```shell
    # Import all RPM GPG keys.
    curl -o /etc/pki/rpm-gpg/google-rpm-package-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
    curl -o /etc/pki/rpm-gpg/google-key.gpg https://packages.cloud.google.com/yum/doc/yum-key.gpg
    rpm --import /etc/pki/rpm-gpg/*
    ```

* IPv6 is disabled in DNF configuration:

    ```shell
    # Disable IPv6 for DNF.
    echo "ip_resolve=4" >> /etc/dnf/dnf.conf
    ```

* Automatic security updates are enabled:

    ```shell
    # Make changes to dnf automatic.conf
    # Apply updates for security (RHEL) by default. NOTE this will not work in CentOS.
    sed -i 's/upgrade_type =.*/upgrade_type = security/' /etc/dnf/automatic.conf
    sed -i 's/apply_updates =.*/apply_updates = yes/' /etc/dnf/automatic.conf
    # Enable the DNF automatic timer service.
    systemctl enable dnf-automatic.timer
    ```

#### Network configuration

* DHCP

    ```shell
    network --bootproto=dhcp --hostname=localhost --device=link
    cat >>/etc/dhcp/dhclient.conf <<EOL
    # Set the dhclient retry interval to 10 seconds instead of 5 minutes.
    retry 10;
    EOL
    ```

* Firewall

    ```shell
    firewall --enabled

    # Configure the network for GCE.
    # Given that GCE users typically control the firewall at the network API level,
    # we want to leave the standard Linux firewall setup enabled but all-open.
    firewall-offline-cmd --set-default-zone=trusted
    ```

#### Time synchronization

* Timezone is set to **UTC**.
* Google's NTP server `metadata.google.internal` is used.

#### Users

* `root` user account is locked and its password is set to an invalid value.

```text
rootpw --iscrypted --lock *
```

#### Services

* Enabled:

    ```text
    sshd
    rngd
    ```

* Disabled:

    ```text
    sshd-keygen@
    ```

  * Note that disabling `sshd-keygen@` generator service has no real effect.

* `initial-setup` package service is disabled

    ```text
    firstboot --disabled
    ```

* Default to `multi-user.target`

    ```text
    skipx
    ```

#### GCE packages configuration

```shell
# Set google-compute-engine config for EL8.
cat >>/etc/default/instance_configs.cfg.distro << EOL
# Disable boto plugin setup.
[InstanceSetup]
set_boto_config = false
EOL
```

#### SSH daemon configuration

```shell
# Disable password authentication by default.
sed -i -e '/^PasswordAuthentication /s/ yes$/ no/' /etc/ssh/sshd_config

# Set ServerAliveInterval and ClientAliveInterval to prevent SSH
# disconnections. The pattern match is tuned to each source config file.
# The $'...' quoting syntax tells the shell to expand escape characters.
sed -i -e $'/^\tServerAliveInterval/d' /etc/ssh/ssh_config
sed -i -e $'/^Host \\*$/a \\\tServerAliveInterval 420' /etc/ssh/ssh_config
sed -i -e '/ClientAliveInterval/s/^.*/ClientAliveInterval 420/' /etc/ssh/sshd_config

# Disable root login via SSH by default.
sed -i -e '/PermitRootLogin yes/s/^.*/PermitRootLogin no/' /etc/ssh/sshd_config
```

#### Blacklist floppy kernel module

```shell
# Blacklist the floppy module.
echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
restorecon /etc/modprobe.d/blacklist-floppy.conf
```

#### Removed files

```shell
# Remove files which shouldn't make it into the image. Its possible these files
# will not exist.
rm -f /etc/boto.cfg /etc/udev/rules.d/70-persistent-net.rules

# Remove ens4 config from installer.
rm -f /etc/sysconfig/network-scripts/ifcfg-ens4
```

#### Restore SELinux context

```shell
# Fix selinux contexts on /etc/resolv.conf.
restorecon /etc/resolv.conf
```

#### Packages

* **Common for all RHEL variants**
  * Included:

    ```text
    @Core
    acpid
    dhcp-client
    dnf-automatic
    net-tools
    openssh-server
    python3
    rng-tools
    tar
    vim

    google-compute-engine
    google-osconfig-agent
    gce-disk-expand

    google-cloud-sdk
    ```

  * Excluded:

    ```text
    alsa-utils
    b43-fwcutter
    dmraid
    eject
    gpm
    irqbalance
    microcode_ctl
    smartmontools
    aic94xx-firmware
    atmel-firmware
    b43-openfwwf
    bfa-firmware
    ipw2100-firmware
    ipw2200-firmware
    ivtv-firmware
    iwl100-firmware
    iwl1000-firmware
    iwl3945-firmware
    iwl4965-firmware
    iwl5000-firmware
    iwl5150-firmware
    iwl6000-firmware
    iwl6000g2a-firmware
    iwl6050-firmware
    kernel-firmware
    libertas-usb8388-firmware
    ql2100-firmware
    ql2200-firmware
    ql23xx-firmware
    ql2400-firmware
    ql2500-firmware
    rt61pci-firmware
    rt73usb-firmware
    xorg-x11-drv-ati-firmware
    zd1211-firmware
    ```

* **RHEL (Bring Your Own License)**
  * Included:

    ```text
    subscription-manager
    ```

* **RHEL (Google's RHUI client)**
  * Included:

    ```text
    google-rhui-client-rhel8
    ```

* **RHEL SAP (Google's RHUI client)**
  * Included:

    ```text
    compat-sap-c++-9
    fence-agents-gce
    libatomic
    libtool-ltdl
    lvm2
    numactl
    numactl-libs
    nfs-utils
    pacemaker
    pcs
    resource-agents
    resource-agents-gcp
    resource-agents-sap
    resource-agents-sap-hana
    rhel-system-roles-sap
    tuned-profiles-sap
    tuned-profiles-sap-hana

    google-rhui-client-rhel8-sap
    ```

### Kickstart files used by Google to build RHEL images

The following kickstart files were generated using Google's tooling and files [available on GitHub][daisy-wf-image_build-el].

#### RHEL (Using Google's RHUI)

```shell
# rhel8-options.cfg

### Anaconda installer configuration.
# Install in text mode.
text --non-interactive
harddrive --partition=sda2 --dir=/
poweroff

# Network configuration
network --bootproto=dhcp --hostname=localhost --device=link

### Disk configuration.
# The bootloader must be set to sdb since sda is the installer.
bootloader --boot-drive=sdb --timeout=0 --append="net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y"
# EFI partitioning, creates a GPT partitioned disk.
clearpart --drives=sdb --all
part /boot/efi --size=200 --fstype=efi --ondrive=sdb
part / --size=100 --grow --ondrive=sdb --label=root --fstype=xfs

### Installed system configuration.
firewall --enabled
services --enabled=sshd,rngd --disabled=sshd-keygen@
skipx
timezone --utc UTC --ntpservers=metadata.google.internal
rootpw --iscrypted --lock *
firstboot --disabled
user --name=gce --lock

# packages.cfg
# Contains a list of packages to be installed, or not, on all flavors.
# The %package command begins the package selection section of kickstart.
# Packages can be specified by group, or package name. @Base and @Core are
# always selected by default so they do not need to be specified.

%packages
acpid
dhcp-client
dnf-automatic
net-tools
openssh-server
python3
rng-tools
tar
vim
-subscription-manager
-alsa-utils
-b43-fwcutter
-dmraid
-eject
-gpm
-irqbalance
-microcode_ctl
-smartmontools
-aic94xx-firmware
-atmel-firmware
-b43-openfwwf
-bfa-firmware
-ipw2100-firmware
-ipw2200-firmware
-ivtv-firmware
-iwl100-firmware
-iwl1000-firmware
-iwl3945-firmware
-iwl4965-firmware
-iwl5000-firmware
-iwl5150-firmware
-iwl6000-firmware
-iwl6000g2a-firmware
-iwl6050-firmware
-kernel-firmware
-libertas-usb8388-firmware
-ql2100-firmware
-ql2200-firmware
-ql23xx-firmware
-ql2400-firmware
-ql2500-firmware
-rt61pci-firmware
-rt73usb-firmware
-xorg-x11-drv-ati-firmware
-zd1211-firmware
%end

%post
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-compute-engine]
name=Google Compute Engine
baseurl=https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
%end
%post --erroronfail
set -x
exec &> /dev/ttyS0
dnf -y install google-rhui-client-rhel8
%end

# Google Compute Engine kickstart config for Enterprise Linux 8.
%onerror
echo "Build Failed!" > /dev/ttyS0
shutdown -h now
%end

%post --erroronfail
set -x
exec &> /dev/ttyS0
# Delete the dummy user account.
userdel -r gce

# Import all RPM GPG keys.
curl -o /etc/pki/rpm-gpg/google-rpm-package-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
curl -o /etc/pki/rpm-gpg/google-key.gpg https://packages.cloud.google.com/yum/doc/yum-key.gpg
rpm --import /etc/pki/rpm-gpg/*

# Configure the network for GCE.
# Given that GCE users typically control the firewall at the network API level,
# we want to leave the standard Linux firewall setup enabled but all-open.
firewall-offline-cmd --set-default-zone=trusted

cat >>/etc/dhcp/dhclient.conf <<EOL
# Set the dhclient retry interval to 10 seconds instead of 5 minutes.
retry 10;
EOL

# Disable IPv6 for DNF.
echo "ip_resolve=4" >> /etc/dnf/dnf.conf

# Set google-compute-engine config for EL8.
cat >>/etc/default/instance_configs.cfg.distro << EOL
# Disable boto plugin setup.
[InstanceSetup]
set_boto_config = false
EOL

# Install GCE guest packages.
dnf install -y google-compute-engine google-osconfig-agent gce-disk-expand

# Install the Cloud SDK package.
dnf install -y google-cloud-sdk

# Send /root/anaconda-ks.cfg to our logs.
cp /run/install/ks.cfg /tmp/anaconda-ks.cfg

# Remove files which shouldn't make it into the image. Its possible these files
# will not exist.
rm -f /etc/boto.cfg /etc/udev/rules.d/70-persistent-net.rules

# Remove ens4 config from installer.
rm -f /etc/sysconfig/network-scripts/ifcfg-ens4

# Disable password authentication by default.
sed -i -e '/^PasswordAuthentication /s/ yes$/ no/' /etc/ssh/sshd_config

# Set ServerAliveInterval and ClientAliveInterval to prevent SSH
# disconnections. The pattern match is tuned to each source config file.
# The $'...' quoting syntax tells the shell to expand escape characters.
sed -i -e $'/^\tServerAliveInterval/d' /etc/ssh/ssh_config
sed -i -e $'/^Host \\*$/a \\\tServerAliveInterval 420' /etc/ssh/ssh_config
sed -i -e '/ClientAliveInterval/s/^.*/ClientAliveInterval 420/' /etc/ssh/sshd_config

# Disable root login via SSH by default.
sed -i -e '/PermitRootLogin yes/s/^.*/PermitRootLogin no/' /etc/ssh/sshd_config

# Update all packages.
dnf -y update

# Make changes to dnf automatic.conf
# Apply updates for security (RHEL) by default. NOTE this will not work in CentOS.
sed -i 's/upgrade_type =.*/upgrade_type = security/' /etc/dnf/automatic.conf
sed -i 's/apply_updates =.*/apply_updates = yes/' /etc/dnf/automatic.conf
# Enable the DNF automatic timer service.
systemctl enable dnf-automatic.timer

# Cleanup this repo- we don't want to continue updating with it.
# Depending which repos are used in build, one or more of these files will not
# exist.
rm -f /etc/yum.repos.d/google-cloud-unstable.repo \
  /etc/yum.repos.d/google-cloud-staging.repo

# Clean up the cache for smaller images.
dnf clean all

# Blacklist the floppy module.
echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
restorecon /etc/modprobe.d/blacklist-floppy.conf

# Generate initramfs from latest kernel instead of the running kernel.
kver="$(ls -t /lib/modules | head -n1)"
dracut -f --kver="${kver}"

# Fix selinux contexts on /etc/resolv.conf.
restorecon /etc/resolv.conf
%end

# Cleanup.
%post --nochroot --log=/dev/ttyS0
set -x
rm -Rf /mnt/sysimage/tmp/*
%end
```

#### RHEL BYOS (Bring your own subscription)

```shell
# rhel8-options.cfg

### Anaconda installer configuration.
# Install in text mode.
text --non-interactive
harddrive --partition=sda2 --dir=/
poweroff

# Network configuration
network --bootproto=dhcp --hostname=localhost --device=link

### Disk configuration.
# The bootloader must be set to sdb since sda is the installer.
bootloader --boot-drive=sdb --timeout=0 --append="net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y"
# EFI partitioning, creates a GPT partitioned disk.
clearpart --drives=sdb --all
part /boot/efi --size=200 --fstype=efi --ondrive=sdb
part / --size=100 --grow --ondrive=sdb --label=root --fstype=xfs

### Installed system configuration.
firewall --enabled
services --enabled=sshd,rngd --disabled=sshd-keygen@
skipx
timezone --utc UTC --ntpservers=metadata.google.internal
rootpw --iscrypted --lock *
firstboot --disabled
user --name=gce --lock

# packages.cfg
# Contains a list of packages to be installed, or not, on all flavors.
# The %package command begins the package selection section of kickstart.
# Packages can be specified by group, or package name. @Base and @Core are
# always selected by default so they do not need to be specified.

%packages
acpid
dhcp-client
dnf-automatic
net-tools
openssh-server
python3
rng-tools
tar
vim
-subscription-manager
-alsa-utils
-b43-fwcutter
-dmraid
-eject
-gpm
-irqbalance
-microcode_ctl
-smartmontools
-aic94xx-firmware
-atmel-firmware
-b43-openfwwf
-bfa-firmware
-ipw2100-firmware
-ipw2200-firmware
-ivtv-firmware
-iwl100-firmware
-iwl1000-firmware
-iwl3945-firmware
-iwl4965-firmware
-iwl5000-firmware
-iwl5150-firmware
-iwl6000-firmware
-iwl6000g2a-firmware
-iwl6050-firmware
-kernel-firmware
-libertas-usb8388-firmware
-ql2100-firmware
-ql2200-firmware
-ql23xx-firmware
-ql2400-firmware
-ql2500-firmware
-rt61pci-firmware
-rt73usb-firmware
-xorg-x11-drv-ati-firmware
-zd1211-firmware
%end

%post
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-compute-engine]
name=Google Compute Engine
baseurl=https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
%end
%post --erroronfail
set -x
exec &> /dev/ttyS0
dnf -y install google-rhui-client-rhel8
%end

# RHEL BYOS
%post --erroronfail
set -x
exec &> /dev/ttyS0
yum -y install subscription-manager
yum -y remove google-rhui-client-*
%end

# Google Compute Engine kickstart config for Enterprise Linux 8.
%onerror
echo "Build Failed!" > /dev/ttyS0
shutdown -h now
%end

%post --erroronfail
set -x
exec &> /dev/ttyS0
# Delete the dummy user account.
userdel -r gce

# Import all RPM GPG keys.
curl -o /etc/pki/rpm-gpg/google-rpm-package-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
curl -o /etc/pki/rpm-gpg/google-key.gpg https://packages.cloud.google.com/yum/doc/yum-key.gpg
rpm --import /etc/pki/rpm-gpg/*

# Configure the network for GCE.
# Given that GCE users typically control the firewall at the network API level,
# we want to leave the standard Linux firewall setup enabled but all-open.
firewall-offline-cmd --set-default-zone=trusted

cat >>/etc/dhcp/dhclient.conf <<EOL
# Set the dhclient retry interval to 10 seconds instead of 5 minutes.
retry 10;
EOL

# Disable IPv6 for DNF.
echo "ip_resolve=4" >> /etc/dnf/dnf.conf

# Set google-compute-engine config for EL8.
cat >>/etc/default/instance_configs.cfg.distro << EOL
# Disable boto plugin setup.
[InstanceSetup]
set_boto_config = false
EOL

# Install GCE guest packages.
dnf install -y google-compute-engine google-osconfig-agent gce-disk-expand

# Install the Cloud SDK package.
dnf install -y google-cloud-sdk

# Send /root/anaconda-ks.cfg to our logs.
cp /run/install/ks.cfg /tmp/anaconda-ks.cfg

# Remove files which shouldn't make it into the image. Its possible these files
# will not exist.
rm -f /etc/boto.cfg /etc/udev/rules.d/70-persistent-net.rules

# Remove ens4 config from installer.
rm -f /etc/sysconfig/network-scripts/ifcfg-ens4

# Disable password authentication by default.
sed -i -e '/^PasswordAuthentication /s/ yes$/ no/' /etc/ssh/sshd_config

# Set ServerAliveInterval and ClientAliveInterval to prevent SSH
# disconnections. The pattern match is tuned to each source config file.
# The $'...' quoting syntax tells the shell to expand escape characters.
sed -i -e $'/^\tServerAliveInterval/d' /etc/ssh/ssh_config
sed -i -e $'/^Host \\*$/a \\\tServerAliveInterval 420' /etc/ssh/ssh_config
sed -i -e '/ClientAliveInterval/s/^.*/ClientAliveInterval 420/' /etc/ssh/sshd_config

# Disable root login via SSH by default.
sed -i -e '/PermitRootLogin yes/s/^.*/PermitRootLogin no/' /etc/ssh/sshd_config

# Update all packages.
dnf -y update

# Make changes to dnf automatic.conf
# Apply updates for security (RHEL) by default. NOTE this will not work in CentOS.
sed -i 's/upgrade_type =.*/upgrade_type = security/' /etc/dnf/automatic.conf
sed -i 's/apply_updates =.*/apply_updates = yes/' /etc/dnf/automatic.conf
# Enable the DNF automatic timer service.
systemctl enable dnf-automatic.timer

# Cleanup this repo- we don't want to continue updating with it.
# Depending which repos are used in build, one or more of these files will not
# exist.
rm -f /etc/yum.repos.d/google-cloud-unstable.repo \
  /etc/yum.repos.d/google-cloud-staging.repo

# Clean up the cache for smaller images.
dnf clean all

# Blacklist the floppy module.
echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
restorecon /etc/modprobe.d/blacklist-floppy.conf

# Generate initramfs from latest kernel instead of the running kernel.
kver="$(ls -t /lib/modules | head -n1)"
dracut -f --kver="${kver}"

# Fix selinux contexts on /etc/resolv.conf.
restorecon /etc/resolv.conf
%end

# Cleanup.
%post --nochroot --log=/dev/ttyS0
set -x
rm -Rf /mnt/sysimage/tmp/*
%end
```

A diff compared to the default RHEL-8 image using Google's RHUI

```diff
--- rhel-8-ks.cfg	2021-04-07 13:23:04.084544305 +0200
+++ rhel-8-byos-ks.cfg	2021-04-07 13:23:04.084544305 +0200
@@ -108,6 +108,14 @@
 dnf -y install google-rhui-client-rhel8
 %end
 
+# RHEL BYOS
+%post --erroronfail
+set -x
+exec &> /dev/ttyS0
+yum -y install subscription-manager
+yum -y remove google-rhui-client-*
+%end
+
 # Google Compute Engine kickstart config for Enterprise Linux 8.
 %onerror
 echo "Build Failed!" > /dev/ttyS0
```

#### RHEL SAP (Using Google's RHUI)

```shell
# rhel8-options.cfg

### Anaconda installer configuration.
# Install in text mode.
text --non-interactive
harddrive --partition=sda2 --dir=/
poweroff

# Network configuration
network --bootproto=dhcp --hostname=localhost --device=link

### Disk configuration.
# The bootloader must be set to sdb since sda is the installer.
bootloader --boot-drive=sdb --timeout=0 --append="net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y"
# EFI partitioning, creates a GPT partitioned disk.
clearpart --drives=sdb --all
part /boot/efi --size=200 --fstype=efi --ondrive=sdb
part / --size=100 --grow --ondrive=sdb --label=root --fstype=xfs

### Installed system configuration.
firewall --enabled
services --enabled=sshd,rngd --disabled=sshd-keygen@
skipx
timezone --utc UTC --ntpservers=metadata.google.internal
rootpw --iscrypted --lock *
firstboot --disabled
user --name=gce --lock

# packages.cfg
# Contains a list of packages to be installed, or not, on all flavors.
# The %package command begins the package selection section of kickstart.
# Packages can be specified by group, or package name. @Base and @Core are
# always selected by default so they do not need to be specified.

%packages
acpid
dhcp-client
dnf-automatic
net-tools
openssh-server
python3
rng-tools
tar
vim
-subscription-manager
-alsa-utils
-b43-fwcutter
-dmraid
-eject
-gpm
-irqbalance
-microcode_ctl
-smartmontools
-aic94xx-firmware
-atmel-firmware
-b43-openfwwf
-bfa-firmware
-ipw2100-firmware
-ipw2200-firmware
-ivtv-firmware
-iwl100-firmware
-iwl1000-firmware
-iwl3945-firmware
-iwl4965-firmware
-iwl5000-firmware
-iwl5150-firmware
-iwl6000-firmware
-iwl6000g2a-firmware
-iwl6050-firmware
-kernel-firmware
-libertas-usb8388-firmware
-ql2100-firmware
-ql2200-firmware
-ql23xx-firmware
-ql2400-firmware
-ql2500-firmware
-rt61pci-firmware
-rt73usb-firmware
-xorg-x11-drv-ati-firmware
-zd1211-firmware
%end

%post
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-compute-engine]
name=Google Compute Engine
baseurl=https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
tee -a /etc/yum.repos.d/google-cloud.repo << EOM
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
%end
%post --erroronfail
set -x
exec &> /dev/ttyS0
dnf -y install google-rhui-client-rhel8-sap
%end

%post --erroronfail
set -x
exec &> /dev/ttyS0

# Configure SAP HANA packages.
SAP_PKGS="
compat-sap-c++-9
fence-agents-gce
libatomic
libtool-ltdl
lvm2
numactl
numactl-libs
nfs-utils
pacemaker
pcs
resource-agents
resource-agents-gcp
resource-agents-sap
resource-agents-sap-hana
rhel-system-roles-sap
tuned-profiles-sap
tuned-profiles-sap-hana
"

dnf install -y ${SAP_PKGS}
%end

# Google Compute Engine kickstart config for Enterprise Linux 8.
%onerror
echo "Build Failed!" > /dev/ttyS0
shutdown -h now
%end

%post --erroronfail
set -x
exec &> /dev/ttyS0
# Delete the dummy user account.
userdel -r gce

# Import all RPM GPG keys.
curl -o /etc/pki/rpm-gpg/google-rpm-package-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
curl -o /etc/pki/rpm-gpg/google-key.gpg https://packages.cloud.google.com/yum/doc/yum-key.gpg
rpm --import /etc/pki/rpm-gpg/*

# Configure the network for GCE.
# Given that GCE users typically control the firewall at the network API level,
# we want to leave the standard Linux firewall setup enabled but all-open.
firewall-offline-cmd --set-default-zone=trusted

cat >>/etc/dhcp/dhclient.conf <<EOL
# Set the dhclient retry interval to 10 seconds instead of 5 minutes.
retry 10;
EOL

# Disable IPv6 for DNF.
echo "ip_resolve=4" >> /etc/dnf/dnf.conf

# Set google-compute-engine config for EL8.
cat >>/etc/default/instance_configs.cfg.distro << EOL
# Disable boto plugin setup.
[InstanceSetup]
set_boto_config = false
EOL

# Install GCE guest packages.
dnf install -y google-compute-engine google-osconfig-agent gce-disk-expand

# Install the Cloud SDK package.
dnf install -y google-cloud-sdk

# Send /root/anaconda-ks.cfg to our logs.
cp /run/install/ks.cfg /tmp/anaconda-ks.cfg

# Remove files which shouldn't make it into the image. Its possible these files
# will not exist.
rm -f /etc/boto.cfg /etc/udev/rules.d/70-persistent-net.rules

# Remove ens4 config from installer.
rm -f /etc/sysconfig/network-scripts/ifcfg-ens4

# Disable password authentication by default.
sed -i -e '/^PasswordAuthentication /s/ yes$/ no/' /etc/ssh/sshd_config

# Set ServerAliveInterval and ClientAliveInterval to prevent SSH
# disconnections. The pattern match is tuned to each source config file.
# The $'...' quoting syntax tells the shell to expand escape characters.
sed -i -e $'/^\tServerAliveInterval/d' /etc/ssh/ssh_config
sed -i -e $'/^Host \\*$/a \\\tServerAliveInterval 420' /etc/ssh/ssh_config
sed -i -e '/ClientAliveInterval/s/^.*/ClientAliveInterval 420/' /etc/ssh/sshd_config

# Disable root login via SSH by default.
sed -i -e '/PermitRootLogin yes/s/^.*/PermitRootLogin no/' /etc/ssh/sshd_config

# Update all packages.
dnf -y update

# Make changes to dnf automatic.conf
# Apply updates for security (RHEL) by default. NOTE this will not work in CentOS.
sed -i 's/upgrade_type =.*/upgrade_type = security/' /etc/dnf/automatic.conf
sed -i 's/apply_updates =.*/apply_updates = yes/' /etc/dnf/automatic.conf
# Enable the DNF automatic timer service.
systemctl enable dnf-automatic.timer

# Cleanup this repo- we don't want to continue updating with it.
# Depending which repos are used in build, one or more of these files will not
# exist.
rm -f /etc/yum.repos.d/google-cloud-unstable.repo \
  /etc/yum.repos.d/google-cloud-staging.repo

# Clean up the cache for smaller images.
dnf clean all

# Blacklist the floppy module.
echo "blacklist floppy" > /etc/modprobe.d/blacklist-floppy.conf
restorecon /etc/modprobe.d/blacklist-floppy.conf

# Generate initramfs from latest kernel instead of the running kernel.
kver="$(ls -t /lib/modules | head -n1)"
dracut -f --kver="${kver}"

# Fix selinux contexts on /etc/resolv.conf.
restorecon /etc/resolv.conf
%end

# Cleanup.
%post --nochroot --log=/dev/ttyS0
set -x
rm -Rf /mnt/sysimage/tmp/*
%end
```

A diff compared to the default RHEL-8 image using Google's RHUI

```diff
--- rhel-8-ks.cfg	2021-04-07 13:23:04.084544305 +0200
+++ rhel-8-sap-ks.cfg	2021-04-07 13:23:04.084544305 +0200
@@ -105,7 +105,35 @@
 %post --erroronfail
 set -x
 exec &> /dev/ttyS0
-dnf -y install google-rhui-client-rhel8
+dnf -y install google-rhui-client-rhel8-sap
+%end
+
+%post --erroronfail
+set -x
+exec &> /dev/ttyS0
+
+# Configure SAP HANA packages.
+SAP_PKGS="
+compat-sap-c++-9
+fence-agents-gce
+libatomic
+libtool-ltdl
+lvm2
+numactl
+numactl-libs
+nfs-utils
+pacemaker
+pcs
+resource-agents
+resource-agents-gcp
+resource-agents-sap
+resource-agents-sap-hana
+rhel-system-roles-sap
+tuned-profiles-sap
+tuned-profiles-sap-hana
+"
+
+dnf install -y ${SAP_PKGS}
 %end
 
 # Google Compute Engine kickstart config for Enterprise Linux 8.
```
