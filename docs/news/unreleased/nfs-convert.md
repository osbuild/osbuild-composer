# RHEL 8.4: enable nfs-convert.service

nfs-convert service is not currently being enabled by the redhat-release 
package but it should be enabled. For now, osbuild-composer will explicitly
enable it for both RHEL 8.4 qcow2 images and CentOS Stream qcow2 images.