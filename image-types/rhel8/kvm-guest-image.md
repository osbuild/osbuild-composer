# KVM Guest Image

This is an image meant to be used as a generic base image for various
virtualization environments.

To be usable in common environments, it is provided in the *qcow2* format and
has *cloud-init* installed and enabled.


## Implementation Choices

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

The environments this image is meant to be used in usually don't require any
specialized firmware. Thus, in order to keep the resulting image size small,
all `-firmware` packages are excluded. An exception is the `linux-firmware`
package, which cannot be excluded because the `kernel` package depends on it.
