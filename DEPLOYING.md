# Deploying osbuild-composer

*osbuild-composer* has currently has to be deployed in a virtual machine. The
[tools](./tools) subdirectory contains various scripts (those starting with
`deploy-`) to deploy it into cloud-init-enabled environemnts. These scripts all
take the form:

```
 ./tools/deploy-<target> <config> <userdata>
```

`<config>` depends on the target (see below). `<userdata>` is either a
cloud-init [cloud-config file](https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data), or a directory containing
this configuration, as documented by [./tools/gen-user-data](./tools/gen-user-data).

## Target: QEMU

`tools/deploy-qemu` takes as `<config>` the path to a qcow2 image and starts a
ephemeral virtual machine using qemu. The qcow2 file is not changed and all
changes to the virtual machine are lost after stopping qemu.

Two ports are forwarded to the host via qemu's [user networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29):
22 → 2222 and 443 → 4430.

See [HACKING.md](./HACKING.md) for how to use this target for running
integration tests locally.

## Target: OpenStack

`tools/deploy-openstack` uses the `openstack` tool (from `python3-openstack`)
to deploy a machine in an OpenStack cluster. It expects that an [OpenStack RC
file](https://docs.openstack.org/newton/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html) was sourced into the running shell:

```
. openstackrc.sh
```

`<config>` has to be a JSON-file containing configuration about what kind of
machine to create. For example:

```json
{
  "name": "composer-instance",
  "image": "fedora-32-x86_64",
  "flavor": "m1.small",
  "network": "my-network-id"
}
```
