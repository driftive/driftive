package config

import (
	"driftive/pkg/gh"
	"flag"
	"github.com/rs/zerolog/log"
)

type Config struct {
	RepositoryUrl  string `json:"repository_url" yaml:"repository_url"`
	Branch         string `json:"branch" yaml:"branch"`
	RepositoryPath string `json:"repository_path" yaml:"repository_path"`
	Concurrency    int    `json:"concurrency" yaml:"concurrency"`

	LogLevel string `json:"log_level" yaml:"log_level"`

	DisableStdoutResult bool   `json:"disable_stdout_result" yaml:"disable_stdout_result"`
	SlackWebhookUrl     string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	GithubToken         string `json:"github_token" yaml:"github_token"`
	GithubContext       *gh.GithubActionContext
}

func validateArgs(repositoryUrl, repositoryPath, branch string) {
	if repositoryUrl == "" && repositoryPath == "" {
		panic("Repository URL or path is required")
	}
	if branch == "" && repositoryPath == "" {
		panic("Branch is required if repository URL is provided")
	}
}

func ParseConfig() Config {
	var repositoryUrl string
	var slackWebhookUrl string
	var branch string
	var repositoryPath string
	var concurrency int
	var logLevel string
	var disableStdoutResult bool
	var githubToken string

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of concurrent projects to check. Defaults to 4.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level. Options: trace, debug, info, warn, error, fatal, panic")
	flag.BoolVar(&disableStdoutResult, "disable-stdout", false, "Disable printing drift results to stdout")
	flag.StringVar(&githubToken, "github-token", "", "Github token")
	flag.Parse()

	validateArgs(repositoryUrl, repositoryPath, branch)

	ghContext, err := gh.ParseGHActionContextEnvVar()

	if err != nil {
		log.Warn().Msgf("Failed to parse github action context. %v", err)
	}

	return Config{
		RepositoryUrl:       repositoryUrl,
		Branch:              branch,
		RepositoryPath:      repositoryPath,
		Concurrency:         concurrency,
		LogLevel:            logLevel,
		DisableStdoutResult: disableStdoutResult,
		SlackWebhookUrl:     slackWebhookUrl,
		GithubToken:         githubToken,
		GithubContext:       ghContext,
	}
}
