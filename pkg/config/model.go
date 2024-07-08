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

	EnableStdoutResult  bool   `json:"stdout_result" yaml:"stdout_result"`
	EnableGithubIssues  bool   `json:"github_issues" yaml:"github_issues"`
	CloseResolvedIssues bool   `json:"close_resolved_issues" yaml:"close_resolved_issues"`
	SlackWebhookUrl     string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	GithubToken         string `json:"github_token" yaml:"github_token"`
	GithubContext       *gh.GithubActionContext
}

// DriftAnalysisConfig is the configuration for drift analysis
type DriftAnalysisConfig struct {
	Projects      []models.Project `json:"projects" yaml:"projects"`
	BasePath      string           `json:"base_path" yaml:"base_path"`
	Concurrency   int              `json:"concurrency" yaml:"concurrency"`
	GithubToken   string           `json:"github_token" yaml:"github_token"`
	GithubContext *gh.GithubActionContext
}
