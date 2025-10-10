package config

import (
	"driftive/pkg/gh"
	"driftive/pkg/models"
)

// DriftiveConfig is the configuration for Driftive CLI
type DriftiveConfig struct {
	RepositoryUrl  string `json:"repository_url" yaml:"repository_url"`
	Branch         string `json:"branch" yaml:"branch"`
	RepositoryPath string `json:"repository_path" yaml:"repository_path"`
	Concurrency    int    `json:"concurrency" yaml:"concurrency"`

	LogLevel string `json:"log_level" yaml:"log_level"`
	ExitCode bool   `json:"exit_code" yaml:"exit_code"`

	EnableStdoutResult           bool   `json:"stdout_result" yaml:"stdout_result"`
	SlackWebhookUrl              string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	GithubToken                  string `json:"github_token" yaml:"github_token"`
	GithubContext                *gh.GithubActionContext
	CreateRemediationPullRequest bool `json:"create_remediation_pull_request" yaml:"create_remediation_pull_request"`

	DriftiveApiUrl string `json:"api_url" yaml:"api_url"`
	DriftiveToken  string `json:"token" yaml:"token"`
}

// DriftAnalysisConfig is the configuration for drift analysis
type DriftAnalysisConfig struct {
	Projects      []models.TypedProject `json:"projects" yaml:"projects"`
	BasePath      string                `json:"base_path" yaml:"base_path"`
	Concurrency   int                   `json:"concurrency" yaml:"concurrency"`
	GithubToken   string                `json:"github_token" yaml:"github_token"`
	GithubContext *gh.GithubActionContext
}
