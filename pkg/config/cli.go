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

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
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

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of concurrent projects to check. Defaults to 4.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level. Options: trace, debug, info, warn, error, fatal, panic")
	flag.BoolVar(&enableStdoutResult, "stdout", true, "Enable printing drift results to stdout")
	flag.StringVar(&githubToken, "github-token", "", "Github token")
	flag.Parse()

	validateArgs(repositoryUrl, repositoryPath, branch)

	zerolog.SetGlobalLevel(utils.ParseLogLevel(logLevel))

	ghContext, err := gh.ParseGHActionContextEnvVar()

	if err != nil {
		log.Warn().Msgf("Failed to parse github action context. %v", err)
	}

	deprecatedFlags := []string{"github-issues:github.enabled", "close-resolved-issues:github.close_resolved", "max-opened-issues:github.max_open_issues"}
	for _, flagPair := range deprecatedFlags {
		flags := strings.Split(flagPair, ":")
		if isFlagPassed(flags[0]) {
			log.Warn().Msgf("[DEPRECATED] %s flag is deprecated and will be removed in the next releases. Please use '%s' in the .driftive.yml file.", flags[0], flags[1])
		}
	}

	return DriftiveConfig{
		RepositoryUrl:      repositoryUrl,
		Branch:             branch,
		RepositoryPath:     strings.TrimSuffix(repositoryPath, utils.PathSeparator),
		Concurrency:        concurrency,
		LogLevel:           logLevel,
		EnableStdoutResult: enableStdoutResult,
		SlackWebhookUrl:    slackWebhookUrl,
		GithubToken:        githubToken,
		GithubContext:      ghContext,
	}
}
