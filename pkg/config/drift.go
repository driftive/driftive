package config

import "driftive/pkg/gh"

type DriftConfig struct {
	Projects      []Project `json:"projects" yaml:"projects"`
	BasePath      string    `json:"base_path" yaml:"base_path"`
	Concurrency   int       `json:"concurrency" yaml:"concurrency"`
	GithubToken   string    `json:"github_token" yaml:"github_token"`
	GithubContext *gh.GithubActionContext
}
