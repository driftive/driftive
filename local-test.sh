#!/bin/bash
export GITHUB_TOKEN=$(gh auth token)
export REPO=leolabs-space/terraform-aws-platform-infrastructure
export GITHUB_CONTEXT=$(gh api -X GET /repos/$(gh repo view $REPO --json nameWithOwner -q '.nameWithOwner') --jq '{repository: .full_name, repository_owner: .owner.login}')
go run . --repo-path ./test/suite3 --github-token $GITHUB_TOKEN # --log-level debug