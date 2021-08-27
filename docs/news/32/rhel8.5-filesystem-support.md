# Add custom file system support for RHEL 8.5

The `weldr` api has been extended to support custom file systems for RHEL 8.5.
Filesystem `mountpoints` and minimum partition `size` can be set under blueprint customizations, as below:

```toml
[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648
```

In addition to the root mountpoint, `/`, the following `mountpoints` and their sub-directories are supported:

- `/var`
- `/home`
- `/opt`
- `/srv`
- `/usr`
- `/app`
- `/data`