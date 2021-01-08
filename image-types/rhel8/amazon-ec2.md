# Amazon EC2 Image

This image is meant to be used in [Amazon Elastic Compute Cloud (EC2)][ec2], a
popular cloud computing platform. It conforms to Amazon’s
[guidelines][guidelines] and [requirements][requirements] for shared AMIs.


## Implementation Choices

EC2 uses Amazon Machine Images (AMIs) internally, which can only be created
inside EC2. An image in a standard format (ova, vmdk, vhd/x, or raw) must be
imported from S3 storage. *osbuild-composer* generates this image type in the
RAW format for the best compatibility with AWS.

This image is available for `x86_64` and `aarch64`, because those are the only
architectures available in EC2.

EC2 doesn't require any specialized firmware. Thus, in order to keep the
resulting image size small, all `-firmware` packages are excluded. An exception
is the `linux-firmware` package, which cannot be excluded because the `kernel`
package depends on it.

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.

The most common way to consume official Red Hat content using this type of
image is via [Red Hat Update Infrastructure (RHUI)][rhui]. RHUI is used mostly
for Red Hat Enterprise Linux (RHEL) content, including some of its extensions
(e.g. for SAP). To access any more specific Red Hat content, one has to use
[Red Hat Subscription Management (RHSM)][rhsm] ([Red Hat Satellite][satellite]
/ Red Hat CDN).

[ec2]: https://aws.amazon.com/ec2
[guidelines]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/building-shared-amis.html
[requirements]: https://docs.aws.amazon.com/vm-import/latest/userguide/vmie_prereqs.html
[rhui]: https://access.redhat.com/products/red-hat-update-infrastructure/
[rhsm]: https://access.redhat.com/products/red-hat-subscription-management
[satellite]: https://access.redhat.com/products/red-hat-satellite/
