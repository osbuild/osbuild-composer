# Weldr API: introduce the ablility to limit exposed Image Types by configuration

Extend Weldr API to accept a map of distribution-specific lists of denied
image types, which should not be exposed via API. A special name `*` can be
used to match any Distribution or any Image Type. This functionality is needed
to not expose image types which can't be successfully built outside
of Red Hat VPN.

The list of denied Image Types is defined in `osbuild-composer` configuration,
`/etc/osbuild-composer/osbuild-composer.toml`.

Example configuration denying the building of `qcow2` and `vmdk` Image Types
via Weldr API for any distribution:
```toml
[weldr_api.distros."*"]
image_type_denylist = [ "qcow2", "vmdk" ]
```

Example configuration denying the building of `qcow2` and `vmdk` Image Types
via Weldr API for `rhel-84` distribution:
```toml
[weldr_api.distros.rhel-84]
image_type_denylist = [ "qcow2", "vmdk" ]
```
