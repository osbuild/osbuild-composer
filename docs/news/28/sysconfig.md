# RHEL 8.4: add support for org.osbuild.sysconfig stage

The kernel and network sysconfigs need to have certain values set in RHEL 8.4.
Currently, the following values are set for all image types in 8.4:

  kernel:
    UPDATEDEFAULT=yes
    DEFAULTKERNEL=kernel
    
  network:
    NETWORKING=yes
    NOZEROCONF=yes
    
