# Building images for other distributions

Previously osbuild-composer could only build images for the same distribution
as the host. With the addition of the distro field in blueprint it is now
possible to build for any supported distribution shipped with osbuild-composer.


## New API route: /distros/list

The API now supports listing the available distributions. It will return a JSON
object listing the installed distro names that can be used by blueprints,
sources, and the optional `?distro=` selection on API routes.

eg. `curl --unix-socket /run/weldr/api.socket http://localhost/api/v1/distros/list`

    {
        "distros": [
            "centos-8",
            "fedora-32",
            "fedora-33",
            "rhel-8",
            "rhel-84",
            "rhel-85",
            "rhel-90"
        ]
    }


## Distribution selection with blueprints

The blueprint now supports a new `distro` field that will be used to select the
distribution to use when composing images, or depsolving the blueprint. If
`distro` is left blank it will use the host distribution. If you upgrade the
host operating system the blueprints with no `distro` set will build using the
new os.

eg. A blueprint that will always build a fedora-32 image, no matter what
version is running on the host:

    name = "tmux"
    description = "tmux image with openssh"
    version = "1.2.16"
    distro = "fedora-32"

    [[packages]]
    name = "tmux"
    version = "*"

    [[packages]]
    name = "openssh-server"
    version = "*"


## Using sources with specific distributions

A new optional field has been added to the repository source format. It is a
list of distribution strings that the source will be used with when depsolving
and building images.

Sources with no `distros` will be used with all composes. If you want to use a
source for a specific distro you set the `distros` list to the distro name(s)
to use it with.

eg. A source that is only used when depsolving or building fedora 32:

    check_gpg = true
    check_ssl = true
    distros = ["fedora-32"]
    id = "f32-local"
    name = "local packages for fedora32"
    system = false
    type = "yum-baseurl"
    url = "http://local/repos/fedora32/projectrepo/"

This source will be used for any requests that specify fedora-32, eg. listing
packages and specifying fedora-32 will include this source, but listing
packages for the host distro will not.


## Optional distribution selection for routes

Many of the API routes now support selecting the distribution to use when
returning results.  Add `?distro=<DISTRO-NAME>` to the API request and it will
return results using `fedora-32` instead of the host distro.

The following routes support distro selection:

* /compose/types
* /modules/list
* /modules/info
* /projects/list
* /projects/info
* /projects/depsolve

The compose start uses the distribution specified by the blueprint to select
which one to use.

eg. Show the image types supported by `centos-8`:

    curl --unix-socket /run/weldr/api.socket http://localhost/api/v1/compose/types?distro=centos-8
    {
        "types": [
            {
                "name": "ami",
                "enabled": true
            },
            {
                "name": "openstack",
                "enabled": true
            },
            {
                "name": "qcow2",
                "enabled": true
            },
            {
                "name": "tar",
                "enabled": true
            },
            {
                "name": "vhd",
                "enabled": true
            },
            {
                "name": "vmdk",
                "enabled": true
            }
        ]
    }


## Unknown Distributions

If an unknown distribution is selected the response from the API server will be
a `DistroError`, like this:

    {
        "status": false,
        "errors": [
            {
                "id": "DistroError",
                "msg": "Invalid distro: fedora-1"
            }
        ]
    }

