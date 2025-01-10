All repository data is imported via the "images" library, see
https://github.com/osbuild/images/tree/main/data/repositories

Most are imported via the "vendor" mechanism, but some special
repositories are kept here, like the `*-no-aux-key.json` that is for
rhel8 only and will only get installed on rhel10 (because the crypto
policy on rhel10 does not allow the sha1 aux-key in the original
rhel8 keys.

It also contains the centos-* -> centos-stream-* symlinks as
`go:embed` does not support symlinks so we need to keep them
externally.
