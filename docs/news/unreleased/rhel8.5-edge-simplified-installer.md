# Add a new Simplified Installer for RHEL for Edge 8.5

OSBuild Composer can now build the RHEL 8.5 for Edge Simplified Installer.
This installer is optimized for unattended installation to a device, which
can be specified via a new blueprint option, `installation_device`. As for
the existing RHEL for Edge installer, an existing OSTree commit needs to
be provided. A raw image will be created with that commit deployed in it
and the installer will flash this raw image to the specified installation
device.
The following image new types are supported: edge-simplified-installer.

Relevant PR: https://github.com/osbuild/osbuild-composer/pull/1654