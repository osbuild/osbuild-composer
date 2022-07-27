#!/usr/bin/env bash

set -euo pipefail

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}
function redprint {
    echo -e "\033[1;31m[$(date -Isecond)] ${1}\033[0m"
}
function revert_to_head {
   git checkout "$head"
}

if [[ "${CI_COMMIT_BRANCH}" != PR-* ]]; then
    greenprint "${CI_COMMIT_BRANCH} is not a Pull Request"
    greenprint "Skipping"
    exit 0
fi

greenprint "Getting PR number"
prnum="${CI_COMMIT_BRANCH#PR-}"

greenprint "Installing jq"
sudo dnf install -y jq

greenprint "Getting base branch name"
basebranch=$(curl  \
    -u "${SCHUTZBOT_LOGIN}" \
    -H 'Accept: application/vnd.github.v3+json' \
    "https://api.github.com/repos/osbuild/osbuild-composer/pulls/${prnum}" | jq -r ".base.ref")

greenprint "Fetching origin/${basebranch}"
git fetch origin "${basebranch}"

greenprint "Getting revision IDs for HEAD and merge-base"
head=$(git rev-parse HEAD)
mergebase=$(git merge-base HEAD "origin/${basebranch}")

if [[ "${head}" == "${mergebase}" ]]; then
    greenprint "HEAD and merge-base are the same"
    greenprint "Test is unnecessary"
    exit 0
fi

# We are compiling things, install the build requirements
greenprint "Installing build dependencies"
# first we need to install the rpm macros so that dnf can parse our spec file
sudo dnf install -y redhat-rpm-config
# we need to have access to codeready-builder repos for the dependencies
sudo dnf config-manager --set-enabled codeready-builder-for-rhel-9-rhui-rpms
# now install our build requirements
sudo dnf build-dep -y osbuild-composer.spec

manifestdir=$(mktemp -d)

greenprint "Generating all manifests for HEAD (PR #${prnum})"
if ! go run ./cmd/gen-manifests --output "${manifestdir}/PR" --workers 50; then
    redprint "Manifest generation on PR HEAD failed"
    exit 1
fi

# revert to $head on exit
trap revert_to_head EXIT
greenprint "Checking out merge-base ${mergebase}"
git checkout "${mergebase}"

greenprint "Generating all manifests for merge-base (${mergebase})"
# NOTE: it's not an error if this task fails; manifest generation on base
# branch can be broken in a PR that fixes it.
# As long as the generation on the PR HEAD succeeds, the job should succeed.
merge_base_fail=""
if ! go run ./cmd/gen-manifests --output "${manifestdir}/${mergebase}" --workers 50; then
    redprint "Manifest generation on merge-base failed"
    merge_base_fail="**NOTE:** Manifest generation on merge-base with \`${basebranch}\` (${mergebase}) failed.\n\n"
fi

greenprint "Diff: ${manifestdir}/${mergebase} ${manifestdir}/PR"
if diff=$(diff -Naur "${manifestdir}"/"${mergebase}" "${manifestdir}/PR"); then
    greenprint "No changes in manifests"
    exit 0
fi

greenprint "Manifests differ"
echo "${diff}" > "manifests.diff"
greenprint "Saved diff in job artifacts"

artifacts_url="${CI_JOB_URL}/artifacts/browse"

review_data_file="review.json"
cat > "${review_data_file}" << EOF
{"body":"⚠️ This PR introduces changes in at least one manifest (when comparing PR HEAD ${head} with the ${basebranch} merge-base ${mergebase}).  Please review the changes.  The changes can be found in the [artifacts of the \`Manifest-diff\` job [0]](${artifacts_url}) as \`manifests.diff\`.\n\n${merge_base_fail}[0] ${artifacts_url}","event":"COMMENT"}
EOF

greenprint "Posting review comment"
comment_req_out=$(mktemp)
comment_status=$(curl \
    -u "${SCHUTZBOT_LOGIN}" \
    -X POST \
    -H "Accept: application/vnd.github.v3+json" \
    --show-error \
    --write-out '%{http_code}' \
    --output "${comment_req_out}" \
    "https://api.github.com/repos/osbuild/osbuild-composer/pulls/${prnum}/reviews" \
    -d @"${review_data_file}")

cat "${comment_req_out}"

if [[ "${comment_status}" != "200" ]]; then
    redprint "Comment post failed (${comment_status})"
    exit 1
fi

exit 0
