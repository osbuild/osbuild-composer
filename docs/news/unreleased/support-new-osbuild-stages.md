# Added support for new osbuild stages required for RHEL EC2 SAP images

Added support for the following osbuild stages:

- `org.osbuild.selinux.config` - configures SELinux policy state and type on the system
- `org.osbuild.tmpfilesd` - creates tmpfiles.d configuration files
- `org.osbuild.pam.limits.conf` - creates configuration files for pam_limits module
- `org.osbuild.sysctld` - creates sysctl.d configuration files
- `org.osbuild.dnf.config` - configures DNF (currently only variables)
- `org.osbuild.tuned` - sets active tuned profile (or more profiles)
