# Add support for RHEL 8.5 main image types

OSBuild Composer can now build RHEL 8.5 images.  The following new image types
are supported:

- `qcow2`
- `vhd`
- `vmdk`
- `openstack`
- `ami`
- `ec2`
- `ec2-ha`

## RHEL-8.5 AWS images

The `ami` image type have been redefined based on the official RHEL EC2 images.

Notable changes compared to RHEL-8.4 are:

- the default user created by cloud-init is `ec2-user`
- NTP client configuration uses `169.254.169.123` NTP server by default
- the boot mode was changed from hybrid to legacy only

The `ec2` and `ec2-ha` images represent the official RHEL EC2 images, which are
produced as part of RHEL release. These contain RHUI client packages, which are
available only from within Red Hat internal network. For this reason, these
image types are by default not exposed via Weldr API (in the on-premise use
case) for all RHEL releases.

This default configuration can be overridden by placing the following line in
the osbuild-composer configuration `/etc/osbuild-composer/osbuild-composer.toml`:

```toml
[weldr_api.distros."rhel-*"]
# no lines below this section
```

## Extended osbuild support
To support these image types, the following new types were added to support the
functionality in osbuild.

Stages:
- org.osbuild.copy
- org.osbuild.truncate
- org.osbuild.sfdisk
- org.osbuild.qemu
- org.osbuild.mkfs.btrfs
- org.osbuild.mkfs.ext4
- org.osbuild.mkfs.fat
- org.osbuild.mkfs.xfs
- org.osbuild.grub2.inst


Devices:
- org.osbuild.loopback

Mounts:
- org.osbuild.btrfs
- org.osbuild.ext4
- org.osbuild.fat
- org.osbuild.xfs
