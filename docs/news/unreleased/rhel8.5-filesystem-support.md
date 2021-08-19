# Add custom file system support for RHEL 8.5

The `weldr` api has been extended to support custom file systems for RHEL 8.5.
Filesystem `mountpoints` and minimum partition `size` can be set under blueprint customizations, as below:

```toml
[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648
```

The following `mountpoints` are supported:

- `/var`
- `/var/*`
- `/home`
- `/opt`
- `/srv`
- `/usr`
- `/`