# Retrieve metadata about a compose through the Cloud API

A new endpoint is available in the Cloud API at `compose/id/metadata`.  This
endpoint returns a full package list (NEVRA) for the image that was built and
the OSTree commit ID for Edge (OSTree) image types.

PR: https://github.com/osbuild/osbuild-composer/pull/1490
