# Amazon EC2 Image

This image is meant to be used in [Amazon Elastic Compute Cloud (EC2)][ec2], a
popular cloud computing platform. It conforms to Amazon’s
[guidelines][guidelines] and [requirements][requirements] for shared AMIs.


## Implementation Choices

EC2 uses Amazon Machine Images (AMIs) internally, which can only be created
inside EC2. An image in a standard format (ova, vmdk, vhd/x, or raw) must be
imported from S3 storage. *osbuild-composer* generates this image type in the
VHDX format, because it is fairly modern, widely used, and uses less space than
RAW images.

This image is availble for `x86_64` and `aarch64`, because those are the only
architectures available in EC2.

EC2 doesn't require any specialized firmware. Thus, in order to keep the
resulting image size small, all `-firmware` packages are excluded. An exception
is the `linux-firmware` package, which cannot be excluded because the `kernel`
package depends on it.

`dracut-config-rescue` is excluded from the image, because the boot loader
entry it provides is broken.


[ec2]: https://aws.amazon.com/ec2
[guidelines]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/building-shared-amis.html
[requirements]: https://docs.aws.amazon.com/vm-import/latest/userguide/vmie_prereqs.html
