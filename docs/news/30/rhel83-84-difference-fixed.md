# RHEL 8.4: Update rhel-84 distro to better match RHEL 8.3

This restores net-tools to the default package set.

In RHEL8.3 cloud-init depended on net-tools, but in RHEL8.4,
the dependency was dropped. We still want net-tools in the
default package set, so add the dependency explicitly.