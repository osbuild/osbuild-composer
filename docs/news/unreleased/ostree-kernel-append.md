# OSTree compose types with kernel boot parameters return error

Previously, specifying Kernel boot parameters in a Blueprint via the
`[customizations.kernel]` section and requesting an OSTree image type
(`rhel-edge-commit` or `fedora-iot-commit`) would produce an image but the boot
parameters would be ignored.

This combination now returns an error message that the configuration is not
supported.
