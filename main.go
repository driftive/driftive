package main

import (
	"driftive/pkg/config"
	"driftive/pkg/config/discover"
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/models"
	"driftive/pkg/notification"
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

// determineRepositoryDir returns the repository path to use. If repositoryPath is provided, it is returned. Otherwise, the repositoryUrl is returned.
// The second return value is true if the repositoryPath should be deleted after the program finishes.
func determineRepositoryDir(repositoryUrl, repositoryPath, branch string) (string, bool) {
	if repositoryPath != "" {
		return repositoryPath, false
	}

	createdDir, err := os.MkdirTemp("", "driftive")
	if err != nil {
		panic(err)
	}

	log.Debug().Msgf("Created temp dir: %s", createdDir)
	err = git.CloneRepo(repositoryUrl, branch, createdDir)
	if err != nil {
		panic(err)
	}
	log.Info().Msgf("Cloned repo: %s to %s", repositoryUrl, createdDir)

	return createdDir, true
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: ""})
	cfg := config.ParseConfig()

	showInitMessage(cfg)

	repoDir, shouldDelete := determineRepositoryDir(cfg.RepositoryUrl, cfg.RepositoryPath, cfg.Branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after driftive finishes.")
		defer os.RemoveAll(repoDir)
	}

	repoConfig, err := config.DetectRepoConfig(repoDir)
	if err != nil && !errors.Is(err, config.ErrMissingRepoConfig) {
		log.Fatal().Msgf("Failed to load repository config. %v", err)
	}

	var projects []models.Project
	if repoConfig != nil {
		log.Info().Msg("Repository config detected")
		projects = discover.AutoDiscoverProjects(repoDir, repoConfig)
	} else {
		log.Info().Msg("No repository config detected. Using default auto-discovery rules.")
		projects = discover.AutoDiscoverProjects(repoDir, config.DefaultRepoConfig())
	}
	log.Info().Msgf("Projects detected: %d", len(projects))
	driftDetector := drift.NewDriftDetector(repoDir, projects, cfg.Concurrency)
	analysisResult := driftDetector.DetectDrift()

	if analysisResult.TotalDrifted > 0 {
		if cfg.SlackWebhookUrl != "" {
			log.Info().Msg("Sending notification to slack...")
			slack := notification.Slack{Url: cfg.SlackWebhookUrl}
			err := slack.Send(analysisResult)
			if err != nil {
				log.Error().Msgf("Failed to send slack notification. %v", err)
			}
		}

		if cfg.EnableStdoutResult {
			stdout := notification.NewStdout()
			err := stdout.Send(analysisResult)
			if err != nil {
				log.Error().Msgf("Failed to print drifts to stdout. %v", err)
			}
		}

		if cfg.EnableGithubIssues && cfg.GithubToken != "" && cfg.GithubContext != nil {
			log.Info().Msg("Sending notification to github...")
			gh := notification.NewGithubIssueNotification(&cfg)
			gh.Send(analysisResult)
		}
	} else {
		log.Info().Msg("No drifts detected")
	}

	if analysisResult.TotalDrifted > 0 {
		os.Exit(1)
	}
}

func parseOnOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func showInitMessage(cfg config.DriftiveConfig) {
	log.Info().Msg("Starting driftive...")
	log.Info().Msgf("Options: concurrency: %d. github issues: %s. slack: %s. close resolved issues: %s. max opened issues: %d",
		cfg.Concurrency,
		parseOnOff(cfg.EnableGithubIssues),
		parseOnOff(cfg.SlackWebhookUrl != ""),
		parseOnOff(cfg.CloseResolvedIssues),
		cfg.MaxOpenedIssues)

	if cfg.EnableGithubIssues && (cfg.GithubToken == "" || cfg.GithubContext == nil || cfg.GithubContext.Repository == "" || cfg.GithubContext.RepositoryOwner == "") {
		log.Fatal().Msg("Github issues are enabled but the required Github token or context is not provided. " +
			"Use the --github-token flag or set the GITHUB_TOKEN environment variable. " +
			"Also, ensure that the GITHUB_CONTEXT environment variable is set in Github Actions.")
	}
}
