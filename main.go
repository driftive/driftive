package main

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/discover"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/git"
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
	ctx := context.Background()

	repoDir, shouldDelete := determineRepositoryDir(cfg.RepositoryUrl, cfg.RepositoryPath, cfg.Branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after driftive finishes.")
		defer os.RemoveAll(repoDir)
	}

	repoConfig, err := config.DetectRepoConfig(repoDir)
	if err != nil && !errors.Is(err, config.ErrMissingRepoConfig) {
		log.Fatal().Msgf("Failed to load repository config. %v", err)
	}
	repoConfig = repoConfigOrDefault(repoConfig)
	repo.ValidateRepoConfig(repoConfig)
	showInitMessage(cfg, repoConfig)

	projects := discover.AutoDiscoverProjects(repoDir, repoConfig)
	log.Info().Msgf("Projects detected: %d", len(projects))
	driftDetector := drift.NewDriftDetector(repoDir, projects, cfg.Concurrency)
	analysisResult := driftDetector.DetectDrift()

	notification.NewNotificationHandler(&cfg, repoConfig).
		HandleNotifications(ctx, analysisResult)

	if analysisResult.TotalDrifted <= 0 {
		log.Info().Msg("No drifts detected")
	} else if cfg.ExitCode {
		os.Exit(1)
	}
}

func repoConfigOrDefault(repoConfig *repo.DriftiveRepoConfig) *repo.DriftiveRepoConfig {
	if repoConfig == nil {
		log.Info().Msg("No repository config detected. Using default auto-discovery rules.")
		return config.DefaultRepoConfig()
	}
	log.Info().Msg("Using detected driftive.y(a)ml configuration.")
	return repoConfig
}

func parseOnOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func showInitMessage(cfg config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) {
	log.Info().Msg("Starting driftive...")
	log.Info().Msgf("Options: concurrency: %d. github issues: %s. slack: %s. close resolved issues: %s. max opened issues: %d",
		cfg.Concurrency,
		parseOnOff(repoConfig.GitHub.Issues.Enabled),
		parseOnOff(cfg.SlackWebhookUrl != ""),
		parseOnOff(repoConfig.GitHub.Issues.CloseResolved),
		repoConfig.GitHub.Issues.MaxOpenIssues)

	if repoConfig.GitHub.Issues.Enabled && (cfg.GithubToken == "" || cfg.GithubContext == nil || cfg.GithubContext.Repository == "" || cfg.GithubContext.RepositoryOwner == "") {
		log.Fatal().Msg("Github issues are enabled but the required Github token or context is not provided. " +
			"Use the --github-token flag or set the GITHUB_TOKEN environment variable. " +
			"Also, ensure that the GITHUB_CONTEXT environment variable is set in Github Actions.")
	}
}
