# Weldr API: introduce the ablility to limit exposed Image Types by configuration

Extend Weldr API to accept a list of denied image types, which should
not be exposed via API for any supported distribution. This functionality is
needed to not expose image types which can't be successfully built outside
of Red Hat VPN.

The list of denied Image Types is defined in `osbuild-composer` configuration,
`/etc/osbuild-composer/osbuild-composer.toml`.

Example configuration denying the building of `qcow2` and `vmdk` Image Types
via Weldr API:
```toml
[weldr_api]
image_type_denylist = [ "qcow2", "vmdk" ]
```
