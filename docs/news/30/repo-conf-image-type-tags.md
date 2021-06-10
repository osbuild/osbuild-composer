# Allow image type-specific repositories using Image Type Tags

The schema of the repository definitions used by *Weldr API*, located in `/usr/share/osbuild-composer/repositories/` or `/etc/osbuild-composer/repositories` is extended with a new field called **`image_type_tags`** and is expected to be an array of strings representing specific image types.

The behavior of how are defined repositories processed and used by  osbuild-composer* is extended in the following way:

1. If the repository definition does not have the `image_type_tags` field specified, then it will be used for building all types of images for a given distribution and architecture. This is how all repository definitions had been used before this change.

1. If the repository definition has the `image_type_tags` field specified and set to a non-empty array of strings, then it will be used **only** for building image types, which names are specified in the array.

An example of a user-defined repository override for Fedora 33 in `/etc/osbuild-composer/repositories/fedora-33.json` follows. In addition to Fedora distribution repositories, it defines an additional repository called `my-custom-repo`, which should be used only for `ami` images built on both architectures.

```json
{
    "x86_64": [
        {
            "name": "fedora",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=fedora-33&arch=x86_64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "updates",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=updates-released-f33&arch=x86_64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "fedora-modular",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=fedora-modular-33&arch=x86_64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "updates-modular",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=updates-released-modular-f33&arch=x86_64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "my-repo",
            "metalink": "https://repos.example.org/f33/x86_64",
            "gpgkey": "...",
            "check_gpg": true,
            "image_type_tags": ["ami"]
        }
    ],
    "aarch64": [
        {
            "name": "fedora",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=fedora-33&arch=aarch64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "updates",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=updates-released-f33&arch=aarch64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "fedora-modular",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=fedora-modular-33&arch=aarch64",
            "gpgkey": "...",
            "check_gpg": true
        },
        {
            "name": "updates-modular",
            "metalink": "https://mirrors.fedoraproject.org/metalink?repo=updates-released-modular-f33&arch=aarch64",
            "gpgkey": "...",
            "check_gpg": true
        }
        {
            "name": "my-repo",
            "metalink": "https://repos.example.org/f33/aarch64",
            "gpgkey": "...",
            "check_gpg": true,
            "image_type_tags": ["ami"]
        }
    ]
}
```
