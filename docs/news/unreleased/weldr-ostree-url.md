# Weldr API: Allow parent OSTree commit to be read from repository

The weldr API for building OSTree based images is extended to optionally take an `url` parameter instead of the current `parent`.

The `parent` parameter contains the OSTree commit SHA of the parent commit when building an update commit. Obtaining this is cumbersome, so instead the `url` of the repository containing the desired parent commit can be specified. In this case, composer will take the current `HEAD` of the given `ref` as the parent.

At most one of `parent` and `url` can be specified in a given compose request.

Before:

    curl --silent \
        --header "Content-Type: application/json" \
        --unix-socket /run/weldr/api.socket \
        http://localhost/api/v1/compose \
        --data "{ \
            \"blueprint_name\": \"foo\", \
            \"compose_type\": \"rhel-edge-commit\", \
            \"ostree\": {\ \
                \"parent\": \"b8a69e5c79be5830bb272356809a52b1660d2013c26f6973d549d0a312a8d21a\", \
                \"ref\": \"fedora/stable/x86_64/iot\" \
            } \
        }"

After:

    curl --silent \
        --header "Content-Type: application/json" \
        --unix-socket /run/weldr/api.socket \
        http://localhost/api/v1/compose \
        --data "{ \
            \"blueprint_name\": \"foo\", \
            \"compose_type\": \"rhel-edge-commit\", \
            \"ostree\": {\ \
                \"url\": \"https://d2ju0wfl996cmc.cloudfront.net/\", \
                \"ref\": \"fedora/stable/x86_64/iot\" \
            } \
        }"

Relevant PRs:
https://github.com/osbuild/osbuild-composer/pull/1235
