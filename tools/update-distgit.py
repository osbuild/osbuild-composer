#!/usr/bin/python3
import argparse
import contextlib
import os
import subprocess
import tempfile
import urllib.request
from datetime import datetime


def read_file(path) -> str:
    """
    Return the content of the file on the given path.
    """
    with open(path) as f:
        return f.read()


def download_github_release_tarball(user: str, project: str, version: str) -> str:
    """
    Download a github release tarball of the given project.
    """
    project_url = f"https://github.com/{user}/{project}"
    tarball_url = f"{project_url}/archive/v{version}.tar.gz"

    local_path = f"{project}-{version}.tar.gz"
    urllib.request.urlretrieve(
        tarball_url,
        local_path
    )

    return local_path


@contextlib.contextmanager
def extracted_tarball(path: str):
    """
    Extract a tarball into a temporary directory.
    Yields the temporary directory.
    """
    with tempfile.TemporaryDirectory() as tempdir:
        subprocess.run(["tar", "-xf", path, "--directory", tempdir], check=True)
        yield tempdir


def merge_specfiles(upstream: str, downstream: str, version: str, author: str):
    """
    Merge the upstream specfile with the changelog from downstream and add a new
    changelog entry.
    """
    upstream_spec_lines = upstream.splitlines()
    downstream_spec_lines = downstream.splitlines()

    # Find where changelog starts in both specfiles
    changelog_start_in_up_spec = upstream_spec_lines.index("%changelog")
    changelog_start_in_down_spec = downstream_spec_lines.index("%changelog")

    # Create a new changelog entry
    date = datetime.now().strftime("%a %b %d %Y")
    changelog = f"""\
* {date} {author} - {version}-1
- New upstream release

"""

    # Join it all together:
    # Firstly, let's take upstream spec file including the %changelog directory
    # Then, put the newly created changelog entry
    # Finally, put there the changelog from the downstream spec file
    merged_lines = upstream_spec_lines[:changelog_start_in_up_spec + 1] + \
                   changelog.splitlines() + \
                   downstream_spec_lines[changelog_start_in_down_spec + 1:]

    return "\n".join(merged_lines) + "\n"


def update_distgit(user: str, project: str, version: str, author: str, pkgtool: str, release: str):
    """
    Update the dist-git for a new release.
    """
    specfile = f"{project}.spec"

    tarball = download_github_release_tarball(user, project, version)

    old_downstream_specfile = read_file(specfile)

    with extracted_tarball(tarball) as path:
        upstream_specfile = read_file(f"{path}/{project}-{version}/{specfile}")

    new_downstream_specfile = merge_specfiles(upstream_specfile, old_downstream_specfile, version, author)

    with open(specfile, "w") as f:
        f.write(new_downstream_specfile)

    release_arg = ["--release", release] if release else []

    subprocess.check_call([pkgtool, *release_arg, "new-sources", tarball])
    subprocess.check_call(["git", "add", ".gitignore", specfile, "sources"])

    commit_message = f"Update to {version}"
    subprocess.check_call(["git", "commit", "-m", commit_message])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--version", metavar="VERSION", type=str, help="version to be released to downstream",
                        required=True)
    parser.add_argument("--author", metavar="AUTHOR", type=str,
                        help="author of the downstream change (format: Name Surname <email@example.com>", required=True)
    parser.add_argument("--pkgtool", metavar="PKGTOOL", type=str, help="fedpkg, centpkg, or rhpkg", required=True)
    parser.add_argument("--release", metavar="RELEASE", type=str, help="distribution release (required only for centpkg)")
    args = parser.parse_args()

    if args.pkgtool not in ["fedpkg", "centpkg", "rhpkg"]:
        raise RuntimeError("--pkgtool must be fedpkg, centpkg, or rhpkg!")

    update_distgit(
        "osbuild",
        "osbuild-composer",
        args.version,
        args.author,
        args.pkgtool,
        args.release,
    )
