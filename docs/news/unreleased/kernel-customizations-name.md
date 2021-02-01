# Blueprint: Kernel name customization

When creating ostree commits, only one kernel package can be installed at a
time, otherwise creating the commit will fail in rpm-ostree.  This prevents
ostree type builds (RHEL for Edge and Fedora IoT) to add alternative kernels,
in particular, the real-time kernel (`kernel-rt`).

Blueprints now support defining the name of the kernel to be used in an image,
through the `customizations.kernel.name` key.  If not specified, the default
`kernel` package is included as before.

Relevant PRs:
https://github.com/osbuild/osbuild-composer/pull/1175
