#!/usr/bin/python3
import json
import sys
import time

import requests

DISTRO_BASEURLS = {
    "fedora-31": ["http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/"],
    "fedora-32": ["http://download.fedoraproject.org/pub/fedora/linux/releases/32/Everything/x86_64/os/"],
    "rhel-8": [
        "http://download.devel.redhat.com/released/RHEL-8/8.2.0/BaseOS/x86_64/os/",
        "http://download.devel.redhat.com/released/RHEL-8/8.2.0/AppStream/x86_64/os/",

    ]
}


def compose_request(distro, koji):
    repositories = [{"baseurl": baseurl} for baseurl in DISTRO_BASEURLS[distro]]

    req = {
        "name": "name",
        "version": "version",
        "release": "release",
        "distribution": distro,
        "koji": {
            "server": koji,
            "task_id": 1
        },
        "image_requests": [{
            "architecture": "x86_64",
            "image_type": "qcow2",
            "repositories": repositories
        }]
    }

    return req


def main(distro):
    cr = compose_request(distro, "https://localhost/kojihub")
    print(json.dumps(cr))

    r = requests.post("https://localhost:8701/compose", json=cr,
                      cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                      verify="/etc/osbuild-composer/ca-crt.pem")
    if r.status_code != 201:
        print("Failed to create compose")
        print(r.text)
        sys.exit(1)

    print(r.text)
    compose_id = r.json()["id"]

    while True:
        r = requests.get(f"https://localhost:8701/compose/{compose_id}",
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


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} DISTRO", file=sys.stderr)
        sys.exit(1)
    main(sys.argv[1])
