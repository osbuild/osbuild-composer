# Cloud API: fix `image_status.status` value for running compose

Previously, the Cloud API endpoint `/v1/compose/{id}` return value's
`image_status.status` for a running worker job was "running", which didn't
comply with the Cloud API specification. Equivalents allowed by the API
specification are "building", "uploading" or "registering".

Return "building" as the `image_status.status` value for a running compose,
instead of "running". Returning the remaining "uploading" and "registering"
values is not yet implemented.
