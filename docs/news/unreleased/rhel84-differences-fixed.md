# RHEL 8.4: Update rhel-84 distro to better match imagefactory's qcow2

There are minor discrepancies between our nightly image and the imagefactory's
qcow2. These differences are mainly in the installed packages, enabled services,
and disabled services. To remedy these differences the following changes have 
been made:

The following packages have been added to our qcow2 image: oddjob, 
oddjob-mkhomedir, psmisc, authselect-compat, rng-tools, dbxtool.


The following services have been enabled: rngd.service, nfs-convert.service.