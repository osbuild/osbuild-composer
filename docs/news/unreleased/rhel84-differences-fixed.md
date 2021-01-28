# RHEL 8.4: Update rhel-84 distro to better match imagefactory's qcow2

There are minor discrepancies between our nightly image and the imagefactory's
qcow2. These differences are mainly in the installed packages, enabled services,
and disabled services. To remedy these differences the following changes have 
been made:

The following packages have been added to our qcow2 image: oddjob, 
oddjob-mkhomedir, psmisc, authselect-compat, rng-tools, dbxtool.

The following packages have been removed from our qcow2 image: 
dnf-plugin-spacewalk, fwupd, nss, and udisks2.

The following services have been enabled: rngd.service, nfs-convert.service.

The following services have been removed/disabled: mdmonitor.service, 
udisks2.service, fwupd-refresh.timer, mdcheck_continue.timer, 
mdcheck_start.timer, and mdmonitor-oneshot.timer.