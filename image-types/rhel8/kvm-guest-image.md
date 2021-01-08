# KVM Guest Image

This is an image meant to be used as a generic base image for KVM
based hypervisors. It is mainly used with [Red Hat Virtualization
(RHV)][rhv] or [Red Hat OpenStack Platform (RHOSP)][rhosp]. Using this
type of image with other types of hypervisors or with cloud providers
is not supported.

To be usable in common environments, it is provided in the *qcow2* format and
has *cloud-init* installed and enabled.


## Implementation Choices

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

The environments this image is meant to be used in usually don't require any
specialized firmware. Thus, in order to keep the resulting image size small,
all `-firmware` packages are excluded. An exception is the `linux-firmware`
package, which cannot be excluded because the `kernel` package depends on it.

The most common way to consume official Red Hat content using this type of
image is via [Red Hat Subscription Management (RHSM)][rhsm] ([Red Hat
Satellite][satellite] / Red Hat CDN).

[rhv]: https://access.redhat.com/products/red-hat-virtualization/
[rhosp]: https://access.redhat.com/products/red-hat-openstack-platform/
[rhsm]: https://access.redhat.com/products/red-hat-subscription-management
[satellite]: https://access.redhat.com/products/red-hat-satellite/
