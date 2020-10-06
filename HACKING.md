# Hacking on osbuild-composer

*osbuild-composer* cannot be run from the source tree, but has to be installed
onto a system. We recommend doing this by building rpms, with:

    make rpm

This will build rpms from the latest git HEAD (remember to commit changes), for
the current operating system, with a version that contains the commit hash. The
packages end up in `./rpmbuild/RPMS/$arch`.

RPMS are easiest to deal with when they're in a dnf repository. To turn this
directory into a dnf repository and serve it on localhost:8000, run:

    createrepo_c ./rpmbuild/RPMS/x86_64
    python3 -m http.server --directory ./rpmbuild/RPMS/x86_64 8000

To start a ephemeral virtual machine using this repository, run:

    tools/deploy-qemu IMAGE tools/deploy/test

`IMAGE` has to be a path to an cloud-init-enabled image matching the host
operating system, because that's what the packages where built for above.

The second argument points to a directory from which cloud-init user-data is
generated (see `tools/gen-user-data` for details). The one given above tries to
mimick what is run on *osbuild-composer*'s continuous integration
infrastructure, i.e., installing `osbuild-composer-tests` and starting the
service.

You can log into the running machine as user `admin`, with the
password `foobar`. Stopping the machine loses all data.

For a quick compile and debug cycle, we recommend iterating code using thorough
unit tests before going through the full workflow described above.
