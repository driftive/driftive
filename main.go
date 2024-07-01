package main

import (
	"driftive/pkg/config"
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/notification"
	"driftive/pkg/utils"
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

	createdDir, err := os.MkdirTemp("", "drifter")
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

	zerolog.SetGlobalLevel(utils.ParseLogLevel(cfg.LogLevel))

	repoDir, shouldDelete := determineRepositoryDir(cfg.RepositoryUrl, cfg.RepositoryPath, cfg.Branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after the program finishes")
		defer os.RemoveAll(repoDir)
	}

	driftDetector := drift.NewDriftDetector(repoDir, cfg.Concurrency)
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

		if cfg.GithubToken != "" && cfg.GithubContext != nil {
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
