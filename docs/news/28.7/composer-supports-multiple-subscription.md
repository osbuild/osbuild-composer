# Support multiple repository subscriptions

RHEL systems can have multiple subscriptions to different repositories.
Each repository can use its certificate authority and require the users
to authenticate with a client-side TLS certificate.

This is common while using Red Hat Satellite, for example.

osbuild-composer can now work with multiple subscriptions that are available
on the host system. If used with a remote worker, the same subscriptions
must be available on both systems.

Relevant PRs:
https://github.com/osbuild/osbuild-composer/pull/1405
https://github.com/osbuild/osbuild/pull/645
