# Add support for new / extended osbuild stages

Add support for the following new osbuild stages:

- `org.osbuild.modprobe` - allows to configure modprobe using configuration files
- `org.osbuild.dracut.conf` - allows to create dracut configuration files
- `org.osbuild.systemd-logind` - allows to create system-logind configuration drop-ins
- `org.osbuild.cloud-init` - allows to configure cloud-init
- `org.osbuild.authselect` - allows to set system identity and auth sources using authselect

Add support for new functionality of existing osbuild stages:

- `org.osbuild.sysconfig` - allows to create network-scripts ifcfg files
- `org.osbuild.systemd` - allows to create `.service` file drop-ins
- `org.osbuild.chrony` - allows to configure NTP `servers` with lower level configuration options
- `org.osbuild.keymap` - allows to configure X11 keyboard layout
