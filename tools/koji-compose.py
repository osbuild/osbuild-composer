#!/usr/bin/python3
# pylint: disable=invalid-name

import argparse
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

    if "cdn.redhat.com" in koji_repository["baseurl"]:
        koji_repository["rhsm"] = True

    return koji_repository


def compose_request(distro, koji, arch):
    with open(f"/usr/share/tests/osbuild-composer/repositories/{distro}.json", encoding="utf-8") as f:
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

    if cloud_target == "azure":
        return {
            # image name is currently not set for Koji composes
            # "image_name": "",
            "location": os.getenv("AZURE_LOCATION"),
            "resource_group": os.getenv("AZURE_RESOURCE_GROUP"),
            "subscription_id": os.getenv("AZURE_SUBSCRIPTION_ID"),
            "tenant_id": os.getenv("AZURE_TENANT_ID")
        }

    if cloud_target == "gcp":
        return {
            # image name is currently not set for Koji composes
            # "image_name": "",
            "bucket": os.getenv("GCP_BUCKET"),
            "region": os.getenv("GCP_REGION"),
            "share_with_accounts": [os.getenv("GCP_API_TEST_SHARE_ACCOUNT")]
        }

    raise RuntimeError(f"unsupported target cloud: {cloud_target}")


def compose_request_cloud_upload(distro, koji, arch, cloud_target, image_type):
    with open(f"/usr/share/tests/osbuild-composer/repositories/{distro}.json", encoding="utf-8") as f:
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


# Client for the Composer API.
class ComposerAPIClient:

    def __init__(self, api_url, refresh_token, auth_server):
        self.api_url = api_url
        self.refresh_token = refresh_token
        self.auth_server = auth_server

    def access_token(self):
        resp = requests.post(self.auth_server + "/token", data={
            "grant_type": "refresh_token",
            "refresh_token": self.refresh_token,
        }, timeout=5)
        if resp.status_code != 200:
            raise RuntimeError(f"failed to refresh token: {resp.text}")
        return resp.json()["access_token"]

    def submit_compose(self, request):
        return requests.post(self.api_url + "/compose", json=request,
                             headers={"Authorization": f"Bearer {self.access_token()}"},
                             timeout=5)

    def compose_status(self, compose_id):
        return requests.get(self.api_url + f"/composes/{compose_id}",
                            headers={"Authorization": f"Bearer {self.access_token()}"},
                            timeout=5)

    def compose_log(self, compose_id):
        return requests.get(self.api_url + f"/composes/{compose_id}/logs",
                            headers={"Authorization": f"Bearer {self.access_token()}"},
                            timeout=5)


def get_parser():
    parser = argparse.ArgumentParser(description="Koji compose test")
    parser.add_argument("distro", metavar="DISTRO", help="Distribution to build")
    parser.add_argument("arch", metavar="ARCH", help="Architecture to build")
    parser.add_argument("--refresh-token", metavar="TOKEN", help="JWT refresh token. " +
                        "If not provided, read from /etc/osbuild-worker/token")
    parser.add_argument("--auth-server", default="http://localhost:8081", help="Auth server URL")
    parser.add_argument("--koji-url", default="https://localhost:4343/kojihub", help="Koji server to use")
    parser.add_argument("--composer-url", default="http://localhost:443/api/image-builder-composer/v2",
                        help="Composer API server to use")

    cloud_upload_group = parser.add_argument_group("Cloud upload options")
    cloud_upload_group.add_argument("cloud_target", metavar="CLOUD-TARGET", nargs="?", help="Cloud target to use")
    cloud_upload_group.add_argument("image_type", metavar="IMAGE-TYPE", nargs="?", help="Image type to build")

    return parser


def main():
    parser = get_parser()
    args = parser.parse_args()

    if args.cloud_target and not args.image_type or not args.cloud_target and args.image_type:
        parser.error("CLOUD-TARGET and IMAGE-TYPE must be used together")

    refresh_token = args.refresh_token
    if not refresh_token:
        with open("/etc/osbuild-worker/token", encoding="utf-8") as f:
            refresh_token = f.read().strip()

    composer_api_client = ComposerAPIClient(args.composer_url, refresh_token, args.auth_server)

    if args.cloud_target is not None:
        cr = compose_request_cloud_upload(args.distro, args.koji_url, args.arch, args.cloud_target, args.image_type)
    else:
        cr = compose_request(args.distro, args.koji_url, args.arch)

    print(json.dumps(cr), file=sys.stderr)

    r = composer_api_client.submit_compose(cr)
    if r.status_code != 201:
        print("Failed to create compose", file=sys.stderr)
        print(r.text, file=sys.stderr)
        sys.exit(1)

    print(r.text, file=sys.stderr)
    compose_id = r.json()["id"]
    print(compose_id)

    while True:
        r = composer_api_client.compose_status(compose_id)
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

        if status == "failure":
            print("compose failed!", file=sys.stderr)
            print(r.text, file=sys.stderr)
            sys.exit(1)

        if status not in ("pending", "running"):
            print(f"unexpected status: {status}", file=sys.stderr)
            print(r.text, file=sys.stderr)
            sys.exit(1)

        time.sleep(10)

    r = composer_api_client.compose_log(compose_id)
    logs = r.json()
    assert "image_builds" in logs
    assert isinstance(logs["image_builds"], list)
    assert len(logs["image_builds"]) == len(cr["image_requests"])


if __name__ == "__main__":
    main()
