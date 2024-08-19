#!/bin/bash

function new_temp_repo {
  cd "$(dirname -- "${BASH_SOURCE[0]}")"
  go build ../cmd/av
  export PATH=$(pwd):$PATH

  export LOCAL_REPO_DIR=$(mktemp -d --tmpdir "local-repo-XXXXXX")
  export REMOTE_REPO_DIR=$(mktemp -d --tmpdir "remote-repo-XXXXXX")

  cd "$REMOTE_REPO_DIR"
  git init --bare
  cd "$LOCAL_REPO_DIR"
  git init --initial-branch=main
  git config user.name "av-test"
  git config user.email "av-test@nonexistent"
  git remote add origin "$REMOTE_REPO_DIR" -m main
  echo "# Hello World" > README.md
  git add README.md
  git commit -m "Initial commit"
  git push origin main

  mkdir .git/av
  cat << EOF > .git/av/av.db
{
  "repository": {
    "id": "R_nonexistent_",
    "owner": "aviator-co",
    "name": "nonexistent"
  }
}
EOF
  cat << EOF > .git/av/config.yml
github:
    token: "dummy_valid_token"
    baseUrl: "https://github.invalid"
EOF
}

function create_commit {
  filename="$1"
  content="$2"
  message="$3"

  echo "$content" > "$filename"
  git add "$filename"
  git commit -m "$message"
}
