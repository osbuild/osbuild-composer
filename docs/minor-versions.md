# RHEL minor version selection

Generally, osbuild-composer supports building images for all supported RHEL versions _at the time of release of the running osbuild-composer version_.

The most common exception to this rule is that support is added for the next minor version release during that version's development period.

## Build request

When a user requests to build an image, they must specify the distro and major release.  Specifying the minor version is only supported for EUS versions.

The same rules for version selection apply for both the Cloud API and the Weldr API (on-prem), with one difference:
The Weldr API does not require the user to specify a version with every request.  Every request that doesn't specify a `distro` is implicitly a request to build the latest supported version of the host's OS and major version.  In other words, the default value for the `distro` key is `rhel-8` on any RHEL 8 system and `rhel-9` on any RHEL 9 system.

## Image minor version

The following examples illustrate the version of the image that will be built based on the requested distro version.

As of writing (2022-07-14), the following requests are valid and build the corresponding image versions.

| Request          |  Image version |
|------------------|----------------|
| `rhel-8`         |  RHEL 8.6      |
| `rhel-84`        |  RHEL 8.4 EUS  |
| `rhel-86`        |  RHEL 8.6      |
| `rhel-9`         |  RHEL 9.0      |
| `rhel-90`        |  RHEL 9.0      |


To illustrate a more interesting case, with the release of 8.7 and 9.1 (expected 2022-11-08), the following is expected.

| Request          |  Image version |
|------------------|----------------|
| `rhel-8`         |  RHEL 8.7      |
| `rhel-84`        |  RHEL 8.4 EUS  |
| `rhel-86`        |  RHEL 8.6 EUS  |
| `rhel-87`        |  RHEL 8.7      |
| `rhel-9`         |  RHEL 9.1      |
| `rhel-90`        |  RHEL 9.0 EUS  |
| `rhel-91`        |  RHEL 9.1      |
