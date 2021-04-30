# Weldr API: Add support for `gce-byos` image type and upload to Google Cloud Platform

Enhance the Weldr API to support building images for Google Compute Engine,
specifically the `gce-byos` image type for `x86_64` architecture. Add also
support to import the built image into Google Cloud Platform as a Compute Engine
image. The GCP credentials at this moment can not be provided via the API, but
must be configured in the *osbuild-worker* configuration file
`/etc/osbuild-worker/osbuild-worker.toml`, such as in the following example:

```toml
[gcp]
credentials = "/etc/osbuild-worker/gcp-credentials.json"
```

The GCP project to use for import is determined from the credentials file and
can not be changed without also changing the credentials file.
