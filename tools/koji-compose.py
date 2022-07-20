#!/usr/bin/python3
import json
import sys
import time
import os

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
            "architecture": arch,
            "image_type": "guest-image",
            "repositories": repositories
        },
        {
            "architecture": arch,
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


def upload_options_by_cloud_target(cloud_target):
    if cloud_target == "aws":
        return {
            # the snapshot name is currently not set for Koji composes
            # "snapshot_name": "",
            "region": os.getenv("AWS_REGION"),
            "share_with_accounts": [os.getenv("AWS_API_TEST_SHARE_ACCOUNT")]
        }
    elif cloud_target == "azure":
        return {
            # image name is currently not set for Koji composes
            # "image_name": "",
            "location": os.getenv("AZURE_LOCATION"),
            "resource_group": os.getenv("AZURE_RESOURCE_GROUP"),
            "subscription_id": os.getenv("AZURE_SUBSCRIPTION_ID"),
            "tenant_id": os.getenv("AZURE_TENANT_ID")
        }
    elif cloud_target == "gcp":
        return {
            # image name is currently not set for Koji composes
            # "image_name": "",
            "bucket": os.getenv("GCP_BUCKET"),
            "region": os.getenv("GCP_REGION"),
            "share_with_accounts": [os.getenv("GCP_API_TEST_SHARE_ACCOUNT")]
        }
    else:
        raise RuntimeError(f"unsupported target cloud: {cloud_target}")


def compose_request_cloud_upload(distro, koji, arch, cloud_target, image_type):
    with open(f"/usr/share/tests/osbuild-composer/repositories/{distro}.json") as f:
        test_repositories = json.load(f)

    repositories = [composer_repository_to_koji_repository(repo) for repo in test_repositories[arch]]
    image_requests = [
        {
            "architecture": arch,
            "image_type": image_type,
            "repositories": repositories,
            "upload_options": upload_options_by_cloud_target(cloud_target)
        },
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


def main():
    koji_url = "https://localhost:4343/kojihub"
    if len(sys.argv) == 3:
        distro, arch = sys.argv[1], sys.argv[2]
        print("Using simple Koji compose with 2 image requests", file=sys.stderr)
        cr = compose_request(distro, koji_url, arch)
    elif len(sys.argv) == 5:
        distro, arch, cloud_target, image_type = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
        print("Using Koji compose with 1 image requests and upload to cloud", file=sys.stderr)
        cr = compose_request_cloud_upload(distro, koji_url, arch, cloud_target, image_type)
    else:
        print(f"usage: {sys.argv[0]} DISTRO ARCH [CLOUD_TARGET IMAGE_TYPE]", file=sys.stderr)
        sys.exit(1)
    print(json.dumps(cr), file=sys.stderr)

    r = requests.post("https://localhost/api/image-builder-composer/v2/compose", json=cr,
                      cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                      verify="/etc/osbuild-composer/ca-crt.pem")
    if r.status_code != 201:
        print("Failed to create compose", file=sys.stderr)
        print(r.text, file=sys.stderr)
        sys.exit(1)

    print(r.text, file=sys.stderr)
    compose_id = r.json()["id"]
    print(compose_id)

    while True:
        r = requests.get(f"https://localhost/api/image-builder-composer/v2/composes/{compose_id}",
                         cert=("/etc/osbuild-composer/worker-crt.pem", "/etc/osbuild-composer/worker-key.pem"),
                         verify="/etc/osbuild-composer/ca-crt.pem")
        if r.status_code != 200:
            print("Failed to get compose status", file=sys.stderr)
            print(r.text, file=sys.stderr)
            sys.exit(1)
        status = r.json()["status"]
        print(status, file=sys.stderr)
        if status == "success":
            print("Compose worked!", file=sys.stderr)
            print(r.text, file=sys.stderr)
            break
        elif status == "failure":
            print("compose failed!", file=sys.stderr)
            print(r.text, file=sys.stderr)
            sys.exit(1)
        elif status != "pending" and status != "running":
            print(f"unexpected status: {status}", file=sys.stderr)
            print(r.text, file=sys.stderr)
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
    main()
