# Amazon EC2 Image

This image is meant to be used in [Amazon Elastic Compute Cloud (EC2)][ec2], a
popular cloud computing platform. It has to follow Amazon’s
[guidelines][guidelines] and [requirements][requirements] for shared AMIs.

## File Format

EC2 uses Amazon Machine Images (AMIs) internally, which can only be created
inside EC2. An image in a standard format (ova, vmdk, vhd/x, or raw) must be
imported from S3 storage. *osbuild-composer* generates this image type in the
VHDX format, because it is fairly modern, widely used, and uses less space than
RAW images.

The default image size is 6 GB.

## Architectures

Limited to `x86_64` and `aarch64`, because those are available on EC2.

## Partitioning and Booting

The image specifies partitions with a MBR partition table, because that is
required by EC2. It contains one bootable partition formatted with RHEL's
default file system `xfs`.

## Packages

`cloud-init` is included in this image, because it is required by EC2.

EC2 doesn't require any specialized firmware. Thus, in order to keep the
resulting image size small, all `-firmware` packages are excluded. An exception
is the `linux-firmware` package, which cannot be excluded because the `kernel`
package depends on it.

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

## Kernel Command Line

`ro console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto`


[ec2]: https://aws.amazon.com/ec2
[guidelines]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/building-shared-amis.html
[requirements]: https://docs.aws.amazon.com/vm-import/latest/userguide/vmie_prereqs.html
