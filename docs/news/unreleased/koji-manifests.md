# Koji API: New endpoint for getting the manifests of a compose job

A new endpoint is available in the Koji API: `GET /compose/{ID}/manifests`.
Returns the manifests for a running or finished compose. Returns one manifest
for each image in the request, in the order they were defined.

Relevant PRs:
https://github.com/osbuild/osbuild-composer/pull/1155
https://github.com/osbuild/osbuild-composer/pull/1165
