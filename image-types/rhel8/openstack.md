# OpenStack Image

This image is meant to be run in an OpenStack environment. It has to conform to
OpenStack’s [image requirements][requirements].

## File Format

This image is OpenStack's native file format is qcow2.

The default image size is 2 GB.

## Partitioning and Booting

The generated images contain one bootable partition formatted with RHEL's
default file system `xfs`.

## Software

`cloud-init` is included in this image, because it is required by OpenStack.

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

## Kernel Command Line

`ro net.ifnames=0`


[requirements]: https://docs.openstack.org/image-guide/openstack-images.html
