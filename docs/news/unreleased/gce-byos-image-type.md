# Image Types: Add new `gce-byos` image type for Google Cloud Platform

Add a new `gce-byos` image type for `x86_64` architecture of RHEL-8.3 and
RHEL-8.4. The image is loosely based on the RHEL Guest image provided by Google
in their marketplace. Google does quite a lot of tweaks to the image, which are
currently not possible with *osbuild*. The main benefit of the image type is
that it uses proper kernel arguments to work with GCE serial console and the
availability of Google guest tools and SDK on the image. The user is expected to
use *subscription-manager* to access RHEL content using the image.
