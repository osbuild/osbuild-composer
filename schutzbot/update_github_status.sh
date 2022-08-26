#!/bin/bash


# Don't overwrite CI statuses on GitHub branches for nightly pipelines
if [[ "$CI_PIPELINE_SOURCE" == "schedule" ]]; then
    exit 0
fi

# if a user is logged in to the runner, wait until they're done
while (( $(who -s | wc -l)  > 0 )); do
    echo "Waiting for user(s) to log off"
    sleep 30
done

if [[ $1 == "start" ]]; then
  GITHUB_NEW_STATE="pending"
  GITHUB_NEW_DESC="I'm currently testing this commit, be patient."
elif [[ $1 == "finish" ]]; then
  GITHUB_NEW_STATE="success"
  GITHUB_NEW_DESC="I like this commit!"
elif [[ $1 == "update" ]]; then
  if [[ $CI_JOB_STATUS == "canceled" ]]; then
    GITHUB_NEW_STATE="failure"
    GITHUB_NEW_DESC="Someone told me to cancel this test run."
  elif [[ $CI_JOB_STATUS == "failed" ]]; then
    GITHUB_NEW_STATE="failure"
    GITHUB_NEW_DESC="I'm sorry, something is odd about this commit."
  else
    exit 0
  fi
else
  echo "unknown command"
  exit 1
fi

curl \
    -u "${SCHUTZBOT_LOGIN}" \
    -X POST \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/osbuild/osbuild-composer/statuses/${CI_COMMIT_SHA}" \
    -d '{"state":"'"${GITHUB_NEW_STATE}"'", "description": "'"${GITHUB_NEW_DESC}"'", "context": "Schutzbot on GitLab", "target_url": "'"${CI_PIPELINE_URL}"'"}'

# ff release branch on github if this ran on main
if [ "$CI_COMMIT_BRANCH" = "main" ] && [ "$GITHUB_NEW_STATE" = "success" ]; then
    git remote add github "https://${SCHUTZBOT_LOGIN#*:}@github.com/osbuild/osbuild-composer.git"
    # || true to ignore non fast-forwards
    git push github "${CI_COMMIT_SHA}:refs/heads/release" || true
fi
