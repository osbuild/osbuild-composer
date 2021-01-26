# RHEL 8.4: Include timedatex in qcow2 images

Timedatex was an excluded package due to an selinux-policy issue that has been
fixed. Therefore, timedatex should be in the qcow2 image we build. Our list of 
excluded packages for RHEL 8.4 was not being included in our nightly builds so 
we did not realize that timedatex was still being excluded. The issue with the 
excluded packages is now fixed and timedatex is now removed from this list.