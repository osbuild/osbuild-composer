# OpenStack Image

This image is meant to be run in an OpenStack environment. It has to conform to
OpenStack’s [image requirements][requirements].


## Implementation Choices

To be usable in OpenStack, it is provided in the *qcow2* format and has
*cloud-init* installed and enabled.

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

The environments this image is meant to be used in usually don't require any
specialized firmware. Thus, in order to keep the resulting image size small,
all `-firmware` packages are excluded. An exception is the `linux-firmware`
package, which cannot be excluded because the `kernel` package depends on it.


[requirements]: https://docs.openstack.org/image-guide/openstack-images.html
