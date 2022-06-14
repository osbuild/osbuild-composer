#!/usr/bin/env bash

set -euo pipefail

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}
function redprint {
    echo -e "\033[1;31m[$(date -Isecond)] ${1}\033[0m"
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
mergebase=$(git merge-base HEAD origin/main)

if [[ "${head}" == "${mergebase}" ]]; then
    greenprint "HEAD and merge-base are the same"
    greenprint "Test is unnecessary"
    exit 0
fi

greenprint "Installing go"
sudo dnf install -y go

manifestdir=$(mktemp -d)

greenprint "Generating all manifests for HEAD (PR #${prnum})"
go run ./cmd/gen-manifests --output "${manifestdir}/PR" --workers 50 > /dev/null

greenprint "Checking out merge-base ${mergebase}"
git checkout "${mergebase}"

greenprint "Generating all manifests for merge-base (${mergebase})"
go run ./cmd/gen-manifests --output "${manifestdir}/${mergebase}" --workers 50 > /dev/null

greenprint "Diff: ${manifestdir}/${mergebase} ${manifestdir}/PR"
diff=$(diff -r "${manifestdir}"/{"${mergebase}",PR})
err=$?

review_data_file="review.json"

if (( err == 0 )); then
    greenprint "No changes in manifests"
    exit 0
fi

greenprint "Manifests differ"
echo "${diff}"
cat > "${review_data_file}" << EOF
{"body":"⚠️ This PR introduces changes in at least one manifest (when comparing PR HEAD ${head} with the main merge-base ${mergebase}).  Please review the changes.","event":"COMMENT"}
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
