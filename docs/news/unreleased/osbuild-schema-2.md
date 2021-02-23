# Support new OSBuild pipelines and RHEL for Edge commit installer image type

OSBuild Composer can now generate Manifests that conform to the new OSBuild
schema.  A new image type is added that takes advantage of the new schema
called `rhel-edge-container`.  This image type creates an OCI container with an
HTTP server that serves the same payload as a `rhel-edge-commit`.

Relevant PR: https://github.com/osbuild/osbuild-composer/pull/1244
