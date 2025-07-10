# Image types definitions in YAML

This directory contains the "image-type" definitions in YAML.

We currently have subdirectories for:
- fedora
- rhel-7, rhel-8, rhel-9, rhel-10 (which also provide CentosOS Stream, Alma, Alma Kitten)


Under each of those directories there is a `distro.yaml` file
that contains all image-types for the given distro.

The image types are defined under `image_types` (and for rhel also in
go-code but that will be generalized soon, it is already for fedora).

## Example distro.yaml

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

### image_config

This maps directly to https://github.com/osbuild/images/blob/v0.154.0/pkg/distro/image_config.go#L18

Conditions can be used and *only* the "shallow_merge" action is supported,
this means that the image_config from the condition will be merged with
the original config (but only as a shallow merge, i.e. only top-levels
that are not already set will be merged).

### partition_table

This maps directly to https://github.com/osbuild/images/blob/v0.154.0/pkg/disk/partition_table.go#L17

Conditions can be used and *only* the "override" action is supported,
this means that the original partition_table is fully replaced with
the one found via the condition.

### package_sets

The package sets describe what packages should be included in the
"os" or "installer" pipelines. Under each keys there is a list of
objects with "include/exclude" sublists (see the example below).

Conditions can be used and *only* the "append" action is supported,
this means that the packages from the conditions is appended to the
original package sets.

### conditions

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
- distro_name
- arch
- version_less_than
- version_equal
- version_greater
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
