# RHEL 8.4: qcow2 images can now be used by older QEMUs

Previously, the guest image for RHEL 8.4 was only usable by QEMU 1.1 and
newer. However, this image should be usable on RHEL 6 that ships an older
version of QEMU. This is now fixed and the guest image can be now used by
QEMU 0.10 and newer.
