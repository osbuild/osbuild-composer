# RHEL8.4: Fix grub2 kernel selection

By marking the kernel we install as the `saved_entry`, we make sure that installing additional/subsequent kernels do not unintentionally change the default kernel to be booted into.

Relevant PR: https://github.com/osbuild/osbuild-composer/pull/1241
