# Hacking on osbuild-composer

## Virtual Machine

*osbuild-composer* cannot be run from the source tree, but has to be installed
onto a system. We recommend doing this by building rpms, with:

```
dnf install fedora-packager
dnf builddep osbuild-composer
make rpm
```

This will build rpms from the latest git HEAD (remember to commit changes), for
the current operating system, with a version that contains the commit hash. The
packages end up in `./rpmbuild/RPMS/$arch`.

RPMS are easiest to deal with when they're in a dnf repository. To turn this
directory into a dnf repository and serve it on localhost:8000, run:

```
createrepo_c ./rpmbuild/RPMS/x86_64
python3 -m http.server --directory ./rpmbuild/RPMS/x86_64 8000
```

To start a ephemeral virtual machine using this repository, run:

```
tools/deploy-qemu IMAGE tools/deploy/test
```

`IMAGE` has to be a path to an cloud-init-enabled image matching the host
operating system, because that's what the packages were built for above.
Note that the Fedora/RHEL cloud images might be too small for some tests
to pass. Run `qemu-img resize IMAGE 10G` to grow them, cloud-init's growpart
module will grow the root partition automatically during boot. 

The second argument points to a directory from which cloud-init user-data is
generated (see `tools/gen-user-data` for details). The one given above tries to
mimic what is run on *osbuild-composer*'s continuous integration
infrastructure, i.e., installing `osbuild-composer-tests` and starting the
service.

The virtual machine uses qemu's [user networking](https://wiki.qemu.org/Documentation/Networking#User_Networking_.28SLIRP.29), forwarding port 22 to
the host's 2222 and 443 to 4430. You can log into the running machine with

```
ssh admin@localhost -p 2222
```

The password is `foobar`. Stopping the machine loses all data.

For a quick compile and debug cycle, we recommend iterating code using thorough
unit tests before going through the full workflow described above.

## Containers

*osbuild-composer* and *osbuild-composer-worker* can be run using Docker
containers. Building and starting containers is generally faster than building
RPMs and installing them in a VM, so this method is more convenient for
developing and testing changes quickly. However, using this method has several
downsides:
- It doesn't build the RPMs so the `.spec` file isn't tested.
- The environment is quite different from production (e.g., installation paths,
  privileges and permissions).
- The setup is not complete for all required services, so some functionality
  isn't available for testing this way (e.g., Koji Hub and all its dependent
  services).

The containers are a good way to quickly test small changes, but before
submitting a Pull Request, it's recommended to run through all the tests using
the [Virtual Machine](#virtual-machine) setup described above.

### Build and run

To build the containers run:

```
docker-compose build
```

To start the containers run:

```
docker-compose up
```

You can send requests to the *osbuild-composer* container by entering the devel
container and running:

```
curl -k --cert /etc/osbuild-composer/client-crt.pem --key /etc/osbuild-composer/client-key.pem https://172.30.0.10:8080/api/composer-koji/v1/status
```

To rebuild the containers after a change, add the `--build` flag to the `docker-compose` command:

```
docker-compose up --build
```

## Shortening the loop

For some components, it is possible to install distribution packages first and then only to replace binaries which may or may not work for smaller changes.

```
systemctl stop osbuild-composer.service osbuild-composer.socket osbuild-local-worker.socket
make build && sudo install -m755 bin/osbuild-composer bin/osbuild-worker /usr/libexec/osbuild-composer/
systemctl start osbuild-composer.socket osbuild-local-worker.socket
```

## Accessing Cloud API

You can use curl to access the Cloud API:

```
curl --unix-socket /run/cloudapi/api.socket -XGET http://localhost/api/image-builder-composer/v2/openapi
```

## Profiling and debugging

To print memory statistics of the composer Go runtime, set `OSBUILD_COMPOSER_MEMORY_STATUS=1` environment variable and observe some key statistics via the status Weldr API call:

```
curl -s --unix-socket /run/weldr/api.socket -XGET http://localhost/api/status | json_reformat
```

Example result:

```
{
    "api": "1",
    "db_supported": true,
    "db_version": "0",
    "schema_version": "0",
    "backend": "osbuild-composer",
    "build": "git-rev:99f5fec6666b6f682133c1dc5c378f9a619fd16b",
    "msgs": [

    ],
    "memory": {
        "alloc": 25448544,
        "total_alloc": 69720968,
        "heap_alloc": 25448544,
        "heap_sys": 37060608,
        "num_gc": 8,
        "next_gc": 31541706,
        "last_gc": 1747894693326907779
    }
}
```

It is possible to enable Go pprof HTTP endpoint which can be used via web browser or `pprof` command line to inspect internals of a running process. To enable this feature, set `OSBUILD_COMPOSER_ENABLE_PPROF=1` environment variable. Note this will slow down the application significantly, only keep this option for necessary time.

An example how to set mentioned variables:

```
mkdir -p /etc/systemd/system/osbuild-composer.service.d/
echo -e "[Service]\nEnvironment=\"OSBUILD_COMPOSER_MEMORY_STATUS=1\"" > /etc/systemd/system/osbuild-composer.service.d/10-memory-stats.conf
echo -e "[Service]\nEnvironment=\"OSBUILD_COMPOSER_ENABLE_PPROF=1\"" > /etc/systemd/system/osbuild-composer.service.d/10-enable-pprof.conf
systemctl daemon-reload
```

## Enforcing garbage collection

To enforce GC run on Weldr API `api/status` calls, set `OSBUILD_COMPOSER_GC_STATUS_RATE` environmental variable to number between 0.0 and 1.0 where 0 means do not schedule at all and 1.0 meaning schedule it every call. This can be a temporary workaround if a memory leak is found until it is fixed.

## Capturing core

It can be useful to install `debuginfo` packages and capture core dump of a running process for further inspection:

    gcore PID

It can be loaded up via

    dlv core /usr/libexec/osbuild-composer/osbuild-composer core.885812

