# Support new OSBuild pipelines and new RHEL for Edge image types

OSBuild Composer can now generate Manifests that conform to the new OSBuild
schema.  Two new image types are added that take advantage of the new schema:

- `rhel-edge-container`: Creates an OCI container with an embedded
  `rhel-edge-commit`.  Running the container starts a web server that serves
  the commit.

- `rhel-edge-installer`: Creates a boot ISO image that embeds a
  `rhel-edge-commit`.  The commit is pulled from a URL during the compose of
  the boot ISO.

Requesting a `rhel-edge-installer` requires specifying a URL, otherwise the
request will fail.  Blueprint customizations have no effect on the boot ISO and
also cause the request to fail if any are specified.

Relevant PR: https://github.com/osbuild/osbuild-composer/pull/1244
