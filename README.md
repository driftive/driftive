<p align="center">
  <img width="300" height="300" src="assets/driftive.png">
</p>

# Driftive

Driftive is a tool for detecting drift in Terragrunt/Terraform/OpenTofu projects.

## Features
* Concurrently analyze multiple projects in a repository
* Slack notifications
* Creates GitHub issues and/or pull requests for detected drifts
* Supports Terraform, Terragrunt, and OpenTofu projects

## Installation

### CLI

Homebrew
```bash
$ brew install driftive/tap/driftive
```

## Usage

### CLI usage
```bash 
$ driftive --help
$ driftive --repo-path /path/to/projects/repo --slack-url https://hooks.slack.com/services/XXXXX/XXXXX/XXXXX
```

### Docker usage
```bash
docker pull driftive/driftive:x.y.z
docker run driftive/driftive:x.y.z --help
```

### GitHub Action
Driftive can be used as a GitHub action. Check it out [here](https://github.com/marketplace/actions/driftive)


### Configuration
#### CLI options
* `--repo-path` - path to the repository directory containing projects (takes precedence over `--repo-url`)
* `--slack-url` - Slack webhook URL for notifications
* `--concurrency` - number of concurrent projects to analyze (default: 4)
* `--log-level` - log level. Available options: `debug`, `info`, `warn`, `error` (default: `info`)
* `--stdout` - log state drifts to stdout (default: `true`)
* `--github-token` - GitHub token for accessing private repositories
* `--repo-url` - URL of the repository containing the projects
* `--branch` - branch to analyze (default: `main`). Required in case of `--repo-url`

#### Repository configuration

Driftive expects a `driftive.yml` file in the root directory of the repository.

It supports the following configuration options:
* `auto_discover` - auto-discover projects in the repository
  * `enabled` - enable auto-discovery
  * `inclusions` - list of glob patterns to include
  * `exclusions` - list of glob patterns to exclude
  * `project_rules` - list of project rules to apply. Project rules are evaluated in the order they are defined. If a file matches multiple patterns, the first matching rule is used.
    * `pattern` - glob pattern to match the files
    * `executable` - executable to use for the files matching the pattern. Supported executables: `terraform`, `terragrunt`, `tofu`
* `github` - GitHub configuration
  * `summary` - create a summary issue
    * `enabled` - enable summary issue. requires issues to be enabled.
    * `issue_title` - title of the summary issue
  * `issues` - GitHub issues configuration
    * `enabled` - enable GitHub issues
    * `close_resolved` - close resolved issues
    * `max_open_issues` - maximum number of drift issues to keep open
    * `errors` - create issues for projects with errors
      * `enabled` - enable GitHub issues for projects with errors
      * `close_resolved` - close resolved issues
      * `max_open_issues` - maximum number of issues to keep open
      * `labels` - list of labels to apply to the issues
  * `pull_requests` - GitHub pull requests configuration
    * `enabled` - enable GitHub pull requests
    * `close_resolved` - close resolved pull requests
    * `max_open_pull_requests` - maximum number of open pull requests to keep
    * `base_branch` - base branch for the pull requests (default: `main`)
    * `labels` - list of labels to apply to the pull requests
* `settings`
  * `skip_if_open_pr` - skip projects with open pull requests
  

Example configuration:
```yaml
auto_discover:
  enabled: true
  inclusions:
    - '**/*.tf'
    - '**/terragrunt.hcl'

  exclusions:
    - '**/modules/**'
    - '**/.terragrunt-cache/**'
    - '**/.terraform/**'
    - '/terragrunt.hcl' # exclude root terragrunt.hcl

  project_rules:
    - pattern: 'terragrunt.hcl'
      executable: 'terragrunt'

    - pattern: "*.tf"
      executable: "terraform"

github:
  summary:
    enabled: true # create a summary issue. It requires issues to be enabled
    issue_title: "Driftive Summary"
  issues:
    enabled: true # create issues for detected drifts
    close_resolved: true
    max_open_issues: 10
    labels:
      - "drift"
    errors:
      enabled: true # create issues for projects with errors
      close_resolved: true
      max_open_issues: 5
      labels:
        - "plan-failed"
settings:
  skip_if_open_pr: true
```

### Github issues
Driftive supports creating GitHub issues for detected drifts. To enable this feature, you need to provide a GitHub token using the `--github-token` and `--github-issues=true` options and have the GITHUB_CONTEXT environment variable set.
In Github actions, you can set the GITHUB_CONTEXT like this:
```yaml
jobs:
  driftive:
    runs-on: ubuntu-latest
    steps:
      - name: Run driftive
        env:
          GITHUB_CONTEXT: ${{ toJson(github) }}
        run: driftive --repo-path=. --github-token=${{ secrets.GITHUB_TOKEN }} --github-issues=true
```

![GitHub issue](/assets/gh_issues.png "GitHub issue")

### Slack notifications

Driftive supports sending notifications to Slack. To enable this feature, you need to provide a Slack webhook URL.
![Slack notification](/assets/slack_notification.png "Slack notification")



