#!/usr/bin/python3
import json
import sys
import time

import requests


# Composer API for Koji uses a slightly different repository format
# that osbuild-composer does in /usr/share/osbuild-composer/repositories.
#
# This function does the conversion.
def composer_repository_to_koji_repository(repository):
    koji_repository = {
        "baseurl": repository["baseurl"]
    }

    if repository.get("check_gpg", False):
        koji_repository["gpgkey"] = repository["gpgkey"]

    return koji_repository


def compose_request(distro, koji, arch):
    with open(f"/usr/share/tests/osbuild-composer/repositories/{distro}.json") as f:
        test_repositories = json.load(f)

    repositories = [composer_repository_to_koji_repository(repo) for repo in test_repositories[arch]]
    image_requests = [
        {
            "architecture": "x86_64",
            "image_type": "guest-image",
            "repositories": repositories
        },
        {
            "architecture": "x86_64",
            "image_type": "aws",
            "repositories": repositories
        }
    ]

    req = {
        "distribution": distro,
        "koji": {
            "server": koji,
            "task_id": 1,
            "name": "name",
            "version": "version",
            "release": "release",
        },
        "image_requests": image_requests
    }

    return req


def main(distro, arch):
    cr = compose_request(distro, "https://localhost:4343/kojihub", arch)
    print(json.dumps(cr))

    r = requests.post("https://localhost/api/image-builder-composer/v2/compose", json=cr,
                      cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                      verify="/etc/osbuild-composer/ca-crt.pem")
    if r.status_code != 201:
        print("Failed to create compose")
        print(r.text)
        sys.exit(1)

    print(r.text)
    compose_id = r.json()["id"]

    while True:
        r = requests.get(f"https://localhost/api/image-builder-composer/v2/composes/{compose_id}",
                         cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                         verify="/etc/osbuild-composer/ca-crt.pem")
        if r.status_code != 200:
            print("Failed to get compose status")
            print(r.text)
            sys.exit(1)
        status = r.json()["status"]
        print(status)
        if status == "success":
            print("Compose worked!")
            print(r.text)
            break
        elif status == "failure":
            print("compose failed!")
            print(r.text)
            sys.exit(1)
        elif status != "pending" and status != "running":
            print(f"unexpected status: {status}")
            print(r.text)
            sys.exit(1)

        time.sleep(10)

    r = requests.get(f"https://localhost/api/image-builder-composer/v2/composes/{compose_id}/logs",
                     cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                     verify="/etc/osbuild-composer/ca-crt.pem")
    logs = r.json()
    assert "image_builds" in logs
    assert type(logs["image_builds"]) == list
    assert len(logs["image_builds"]) == len(cr["image_requests"])


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} DISTRO ARCH", file=sys.stderr)
        sys.exit(1)
    main(sys.argv[1], sys.argv[2])
