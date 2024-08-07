package config

import (
	"driftive/pkg/gh"
	"driftive/pkg/utils"
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"strings"
)

func validateArgs(repositoryUrl, repositoryPath, branch string) {
	if repositoryUrl == "" && repositoryPath == "" {
		panic("Repository URL or path is required")
	}
	if branch == "" && repositoryPath == "" {
		panic("Branch is required if repository URL is provided")
	}
}

func ParseConfig() DriftiveConfig {
	var repositoryUrl string
	var slackWebhookUrl string
	var branch string
	var repositoryPath string
	var concurrency int
	var logLevel string
	var enableStdoutResult bool
	var githubToken string
	var enableGithubIssues bool
	var closeResolvedIssues bool
	var maxOpenedIssues int

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of concurrent projects to check. Defaults to 4.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level. Options: trace, debug, info, warn, error, fatal, panic")
	flag.BoolVar(&enableStdoutResult, "stdout", true, "Enable printing drift results to stdout")
	flag.StringVar(&githubToken, "github-token", "", "Github token")
	flag.BoolVar(&enableGithubIssues, "github-issues", false, "Enable creating Github issues for drifts, if running in Github Actions.")
	flag.BoolVar(&closeResolvedIssues, "close-resolved-issues", false, "Close Github issues when drifts are resolved. Requires github-issues flag to be set.")
	flag.IntVar(&maxOpenedIssues, "max-opened-issues", 10, "Max number of github issues to open. Defaults to 10.")
	flag.Parse()

	validateArgs(repositoryUrl, repositoryPath, branch)

	zerolog.SetGlobalLevel(utils.ParseLogLevel(logLevel))

	ghContext, err := gh.ParseGHActionContextEnvVar()

	if err != nil {
		log.Warn().Msgf("Failed to parse github action context. %v", err)
	}

	return DriftiveConfig{
		RepositoryUrl:       repositoryUrl,
		Branch:              branch,
		RepositoryPath:      strings.TrimSuffix(repositoryPath, utils.PathSeparator),
		Concurrency:         concurrency,
		LogLevel:            logLevel,
		EnableStdoutResult:  enableStdoutResult,
		EnableGithubIssues:  enableGithubIssues,
		CloseResolvedIssues: closeResolvedIssues,
		MaxOpenedIssues:     maxOpenedIssues,
		SlackWebhookUrl:     slackWebhookUrl,
		GithubToken:         githubToken,
		GithubContext:       ghContext,
	}
}
