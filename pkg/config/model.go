package config

import (
	"driftive/pkg/gh"
)

// DriftiveConfig is the configuration for Driftive CLI
type DriftiveConfig struct {
	RepositoryUrl  string `json:"repository_url" yaml:"repository_url"`
	Branch         string `json:"branch" yaml:"branch"`
	RepositoryPath string `json:"repository_path" yaml:"repository_path"`
	Concurrency    int    `json:"concurrency" yaml:"concurrency"`

	LogLevel string `json:"log_level" yaml:"log_level"`
	ExitCode bool   `json:"exit_code" yaml:"exit_code"`

	EnableStdoutResult bool   `json:"stdout_result" yaml:"stdout_result"`
	SlackWebhookUrl    string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	GithubToken        string `json:"github_token" yaml:"github_token"`
	GithubContext      *gh.GithubActionContext

	DriftiveApiUrl string `json:"api_url" yaml:"api_url"`
	DriftiveToken  string `json:"token" yaml:"token"`
}

// DriftiveAPIEnabled reports whether the Driftive API can be reached. Live progress reporting and
// the terminal upload share this gate so the two cannot drift apart.
func (c *DriftiveConfig) DriftiveAPIEnabled() bool {
	return c.DriftiveToken != "" && c.DriftiveApiUrl != ""
}
