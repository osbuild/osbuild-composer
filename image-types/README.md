# OSBuild Composer Image Types

This directory contains high-level descriptions of all image types that
*osbuild-composer* generates, grouped by operating system.

Each of them contains a short description of the purpose and intended use case
of the image type, and any peculiarities that make this image type differ from
a standard installation.

## minimal-raw Image type

This image type is basically a pre-canned, bootable, minimal rpm image.
From RHEL PoV this will enable us to support arm or similar devices as part
of the SystemReadyIR/ES "Edge on Arm" initiative. In cases where the customer
may not wish to run R4E or to use for development/testing/prototyping where a
ostree image may not be the most straight forward way to do things, `minimal-raw`
will be useful.


The `minimal-raw` image generated using Image Builder will be compressed in
`xz` format. User needs to uncompress it to be able to boot it. User can `dd`
the uncompressed image to any bootable device such as an SD card.

``` bash

xz -d <uuid-minimal-raw.img.xz> -o <minimal-raw.img>

```