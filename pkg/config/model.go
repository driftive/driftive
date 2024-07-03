package config

import "driftive/pkg/gh"

// DriftiveConfig is the configuration for Driftive CLI
type DriftiveConfig struct {
	RepositoryUrl  string `json:"repository_url" yaml:"repository_url"`
	Branch         string `json:"branch" yaml:"branch"`
	RepositoryPath string `json:"repository_path" yaml:"repository_path"`
	Concurrency    int    `json:"concurrency" yaml:"concurrency"`

	LogLevel string `json:"log_level" yaml:"log_level"`

	EnableStdoutResult bool   `json:"stdout_result" yaml:"stdout_result"`
	EnableGithubIssues bool   `json:"github_issues" yaml:"github_issues"`
	SlackWebhookUrl    string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	GithubToken        string `json:"github_token" yaml:"github_token"`
	GithubContext      *gh.GithubActionContext
}

// DriftAnalysisConfig is the configuration for drift analysis
type DriftAnalysisConfig struct {
	Projects      []Project `json:"projects" yaml:"projects"`
	BasePath      string    `json:"base_path" yaml:"base_path"`
	Concurrency   int       `json:"concurrency" yaml:"concurrency"`
	GithubToken   string    `json:"github_token" yaml:"github_token"`
	GithubContext *gh.GithubActionContext
}

type AutoDiscoverRule struct {
	Pattern    string `json:"pattern" yaml:"pattern"`
	Executable string `json:"executable" yaml:"executable" validate:"omitempty,oneof=terraform tofu terragrunt"`
}

// DriftiveRepoConfig is used to configure driftive for a repository.
// It may be defined in a .driftive.yaml file in the repository or passed via environment variable.
type DriftiveRepoConfig struct {
	AutoDiscover DriftiveRepoConfigAutoDiscover `json:"auto_discover" yaml:"auto_discover"`
}

// DriftiveRepoConfigAutoDiscover is used to configure auto discovery of projects in a repository
type DriftiveRepoConfigAutoDiscover struct {
	// Enabled is used to enable or disable auto discovery
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Inclusions list of glob patterns to include in auto discovery
	Inclusions []string `json:"inclusions" yaml:"inclusions"`
	// Exclusions list of glob patterns to exclude in auto discovery
	Exclusions []string `json:"exclusions" yaml:"exclusions"`
	// ProjectRules list of rules to apply to auto discovered projects
	ProjectRules []AutoDiscoverRule `json:"project_rules" yaml:"project_rules"`
}

type ProjectType int

const (
	Terraform ProjectType = iota
	Tofu
	Terragrunt
)

// Project represents a TF/Tofu/Terragrunt project to be analyzed
type Project struct {
	Dir  string      `json:"dir" yaml:"dir"`
	Type ProjectType `json:"type" yaml:"type"`
}
