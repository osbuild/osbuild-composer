# Image types definitions in YAML

This directory contains the supported distributions and the image
definitions in YAML that are used by the "images" library to build
disk or installer images for rpm based distributions.

## Overview

The definitions start with a "distros.yaml" file that contains details
about the supported distributions and distribution releases.

Note that in order to be available a distribution needs an auxiliary
repository JSON file under `./data/repositories` (or in a system
search path) that matches the canonical distribution name
(e.g. rhel-9.6.json or fedora-43.json).

The most simple `distros.yaml` file looks like this:
```yaml
distros:
  - name: simonos-1
    defs_path: ./simonos-1
```

This instructs the "images" library that the distro named
"simonos" with version "1" has its image definitions
under the subdirectory "simonos-1".

The library will search under `defs_path` for a file called
`imagetypes.yaml` that lists the available image types for
the given distro. Note that multiple distributions can point
to the same image defintions (e.g. rhel and centos do this)
and the `imagestypes.yaml` will contain conditions/templates
to match/substitute based on the distro name. This allows
re-use of existing imagetypes when only small tweaks are
needed.

The most simple `simonos-1/imagetypes.yaml` file looks like this:
```yaml
image_types:
  container:
    filename: container.tar
    image_func: container
    exports: ["container"]
    platforms:
      - arch: "x86_64"
    package_set:
      os:
        - include:
            - bash
```
With that the "images" library can now create a "container"
image type that is available on x86_64 and only adds "bash".

Now only a `data/repositories/simonos-1.json` file is needed
to make a complete new distro.

## Existing defintions

### distros.yaml

The existing `distros.yaml` contains:
- fedora
- rhel-{7,8,9,10}
- centos-{8,9,10}

#### Example of a real distros.yaml snippet

```yaml
  - name: "rhel-{{.MajorVersion}}.{{.MinorVersion}}"
    match: 'rhel-10\.[0-9]{1,2}'
    distro_like: rhel-10
    product: "Red Hat Enterprise Linux"
    os_version: "10.{{.MinorVersion}}"
    release_version: 10
    module_platform_id: "platform:el10"
    vendor: "redhat"
    ostree_ref_tmpl: "rhel/10/%s/edge"
    default_fs_type: "xfs"
    defs_path: rhel-10
    iso_label_tmpl: "RHEL-{{.Distro.MajorVersion}}-{{.Distro.MinorVersion}}-0-BaseOS-{{.Arch}}"
    runner:
      name: "org.osbuild.rhel{{.MajorVersion}}{{.MinorVersion}}"
      build_packages: &rhel10_runner_build_packages
        - ..
    conditions:
      "some image types are rhel-only":
        when:
          not_distro_name: "rhel"
        ignore_image_types:
          - azure-cvm
          - ...
    # rhel & centos share the same list of allowed profiles so a
    # single allow list can be used
    oscap_profiles_allowlist: &oscap_profile_allowlist_rhel
      - "xccdf_org.ssgproject.content_profile_anssi_bp28_enhanced"
      - ...
    bootstrap_containers:
      x86_64: "registry.access.redhat.com/ubi{{.MajorVersion}}/ubi:latest"
```

Common keys:

#### name

This is the distribution name in the canonical
`<name>-<major>{,.<minor>}` form. E.g. `fedora-43` or `rhel-8.10`.
The name can contain go templates and the following variables are
supported "Name", "MajorVersion", "MinorVersion". This is useful
when combined with the `match` key (see below). 

#### match

This key allows dynamic matching of distribution names. This is useful
when image types are shared accross multiple minor versions of a
distribution. In the example above `match` will match all names in the
range `rhel-10.0` to `rhel-10.99`. In other words, this file matches
all potential minor versions of RHEL 10.

#### product

A string that describes the product. This is displayed in the
installer image and in the bootloader.

#### os_version

This is used to construct the release string.

#### release_version

This is the `{{.MajorVersion}}` everywhere currently and
will probably be removed in a future update.

#### module_platform_id

The module_platform_id is used in the DNF resolving.

#### vendor

The vendor of the distribution. This is also used in the
bootloader UEFI setup.

#### default_fs_type

The default filesystem for OS and data partitions. This defines the
default filesystem type for the distribution and is as the fallback
for filesystems in the partition table that don't specify a type.

#### iso_label_tmpl

The string that is used to generate the ISO label. The
label can contain go templates. The following templates
are supported: ".Distro.{Name,MajorVersion,MinorVersion}",
".Product", ".Arch", ".ISOLabel".

#### conditions

Conditions to evaluate for the given distribution. This
can be used to change the behavior based on the distro
name, version or similar conditions (see below for a
list of the conditions). Currently the only operation
that is supported is the `ignore_image_types` key.

With that key certain image types can be skipped when
the condition(s) are met. This is useful to e.g. ensure
that certain image types are only available after a
specific distribution version added support for them
(e.g. rhel-9.6+ is required to build `azure-cvm`).

#### bootstrap_containers

Having this allows experimental cross architecture building.
A container in the target architecture to bootstrap the
build process is required here.


### imagetypes.yaml

The image types are defined in the following subdirectories:
- fedora
- rhel-7, rhel-8, rhel-9, rhel-10 (which also provide CentosOS Stream, Alma, Alma Kitten)

Under each of those directories there is a `imagetypes.yaml` file
that contains all image-types for the given distro.

#### Example for a real imagetypes.yaml snippet

```yaml
image_types:
  qcow2: &qcow2
    image_config: &qcow2_image_config
      default_target: "multi-user.target"
      kernel_options: ["console=tty0", "console=ttyS0,115200n8", "no_timer_check"]
      conditions:
        "tweak the rhsm config on rhel":
          when:
            distro_name: "rhel"
          shallow_merge:
            rhsm_config:
              "no-subscription":
                dnf_plugin:
                  product_id:
                    enabled: false
                  subscription_manager:
                    enabled: false
    partition_table:
      <<: *default_partition_tables
    package_sets:
      os:
        - &qcow2_pkgset
          include:
            - "@core"
            - ...
          exclude:
            - "aic94xx-firmware"
            - ...
          conditions:
            "add insights pkgs on rhel":
              when:
                distro_name: "rhel"
              append:
                include:
                  - "insights-client"
                  - "subscription-manager-cockpit"
```

Common keys:

#### image_config

This maps directly to https://github.com/osbuild/images/blob/v0.154.0/pkg/distro/image_config.go#L18

Conditions can be used and *only* the "shallow_merge" action is supported,
this means that the image_config from the condition will be merged with
the original config (but only as a shallow merge, i.e. only top-levels
that are not already set will be merged).

#### partition_table

This maps directly to https://github.com/osbuild/images/blob/v0.154.0/pkg/disk/partition_table.go#L17

Conditions can be used and *only* the "override" action is supported,
this means that the original partition_table is fully replaced with
the one found via the condition.

#### package_sets

The package sets describe what packages should be included in the
"os" or "installer" pipelines. Under each keys there is a list of
objects with "include/exclude" sublists (see the example below).

Conditions can be used and *only* the "append" action is supported,
this means that the packages from the conditions is appended to the
original package sets.

#### platforms_override

This can be used to override the platforms for the image type based
on some condition. See the rhel-8 "ami" image type for an example
where the `aarch64` architecture is only available for rhel-8.9+.

#### conditions

Conditions are expressed using the following form:
```yaml
conditions:
  <<: *shared_conditions
  "some unique description":
    when:
      distro_name: "rhel"
      arch: "x86_64"
      version_less_than: "9.2"
      version_equal: "9.1"
      version_greater_or_equal: "9.3"
    action:
      # for "image_config" types only shallow_merge is supported
      shallow_merge:
        ...
      # for "partition_tables" types only "override" is supported
      override:
        ...
      # for "package_sets" types only "append" is supported
      append:
        ...
```
Conditions are a "map" in YAML so that they can be easily
shared and merge via the  `<<:` operation.

The `when` part of the condition can contain one or more
of:
- `distro_name`
- `not_distro_name`
- `arch`
- `version_less_than`
- `version_equal`
- `version_greater`
If multiple conditions are given under `when` they are
considered logical AND and only if they all match is
the condition executed.

### Pitfalls

All conditions will be evaluated, there is no ordering.

This means one needs to be careful about having something
like:
```yaml
conditions:
 "f40plus kernel options":
 when:
   version_greater_or_equal: 40
 action:
   shallow_merge:
     kernel_options:
       - f40opts
 "f41plus kernel options":
 when:
   version_greater_or_equal: 41
 action:
   shallow_merge:
     kernel_options:
       - f41opts
```
On fedora 42 both conditions will be executed but the
order is random. Because the merge is shallow only
`kernel_options` will have been set by one of the
conditions and it will either be f41opts or f40opts.

In a situation like this use either: `version_equal`
or:
```yaml
 "f40-42 kernel options":
 when:
   version_greater_or_equal: 40
   version_less_than: 43
 action:
   shallow_merge:
     kernel_options:
       - f40,41,42opts
```
