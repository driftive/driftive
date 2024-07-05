<p align="center">
  <img width="300" height="300" src="assets/driftive.png">
</p>

# Driftive

Driftive is a simple tool for detecting drift in Terragrunt/Terraform projects.
Currently, it supports only Terragrunt projects.

## Features
* Concurrently analyze multiple projects
* Notifies about drifts in Slack

## Installation

### CLI

Homebrew
```bash
$ brew install driftive/tap/driftive
```

## Usage

CLI usage:
```bash 
$ driftive --help
$ driftive --repo-path /path/to/terragrunt/projects --slack-url https://hooks.slack.com/services/XXXXX/XXXXX/XXXXX
```

Docker usage:
```bash
docker pull driftive/driftive:x.y.z
docker run driftive/driftive:x.y.z --help
```

### Configuration
#### CLI options
* `--repo-path` - path to the repository directory containing projects (takes precedence over `--repo-url`)
* `--slack-url` - Slack webhook URL for notifications
* `--concurrency` - number of concurrent projects to analyze (default: 4)
* `--log-level` - log level. Available options: `debug`, `info`, `warn`, `error` (default: `info`)
* `--stdout` - log state drifts to stdout (default: `true`)
* `--github-token` - GitHub token for accessing private repositories
* `--github-issues` - create GitHub issues for detected drifts
* `--repo-url` - URL of the repository containing the projects

#### Repository configuration
Driftive uses a configuration file named `driftive.yml` to define the projects to analyze. 
The configuration file should be placed in the root directory of the repository.
With the configuration file, you can define the projects to analyze, the executables to use, 
and the paths to include/exclude.

The `project_rules` section defines the executables to use for the files matching the pattern.
`project_rules` are evaluated in the order they are defined. 
If a file matches multiple patterns, the first matching rule is used.

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
```

### Slack notifications

Driftive supports sending notifications to Slack. To enable this feature, you need to provide a Slack webhook URL.
![Slack notification](/assets/slack_notification.png "Slack notification")



