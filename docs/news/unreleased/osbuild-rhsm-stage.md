# Add support for `org.osbuild.rhsm` osbuild stage

Add support for `org.osbuild.rhsm` osbuild stage. This stage is available in
osbuild since version 24. The stage currently allows only configuring the
enablement status of two RHSM DNF plugins, specifically of `product-id` and
`subscription-manager` DNF plugins.

# RHEL 8.3 & 8.4: Disable all RHSM DNF plugins on qcow2 image

Disable both available RHSM DNF plugins (`product-id` and
`subscription-manager`) on rhel-8 and rhel-84 qcow2 images. The reason for
disabling these DNF plugins is to make the produced images consistent in this
regard, with what had been previously produced by the imagefactory.
