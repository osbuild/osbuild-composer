# Red Hat Enterprise Linux 8

Owner: *None*

*osbuild-composer* can create [Red Hat Enterprise Linux 8][rhel] images.

## Architectures

RHEL supports `x86_64`, `aarch64`, `ppc64le`, and `s390x`.

## Partitioning and Booting

The default file system is `xfs` on one bootable partition.

## Packages

By default, these images are created by installing the `@Core` group and the
`kernel` package.


[rhel]: https://access.redhat.com/products/red-hat-enterprise-linux
