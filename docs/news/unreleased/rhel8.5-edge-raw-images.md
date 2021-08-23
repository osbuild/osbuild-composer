# Add RHEL for Edge Raw Images for 8.5

OSBuild Composer can now build the RHEL 8.5 Raw Images. This images are
compressed raw images, i.e. a file that has a partition layout with an
deployed OSTree commit in it. It can be used to flash onto a hard drive
or booted in a virtual machine. An existing OSTree commit needs to
be provided.
The following image new types are supported: edge-raw-image.

Relevant PR: https://github.com/osbuild/osbuild-composer/pull/1667
