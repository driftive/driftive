package main

import (
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/notification"
	"driftive/pkg/utils"
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"time"
)

func validateArgs(repositoryUrl, repositoryPath, slackWebhookUrl, branch string) {
	if repositoryUrl == "" && repositoryPath == "" {
		panic("Repository URL or path is required")
	}
	if branch == "" && repositoryPath == "" {
		panic("Branch is required if repository URL is provided")
	}
}

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
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	var repositoryUrl string
	var slackWebhookUrl string
	var branch string
	var repositoryPath string
	var concurrency int
	var logLevel string

	flag.StringVar(&repositoryPath, "repo-path", "", "Path to the repository. If provided, the repository will not be cloned.")
	flag.StringVar(&repositoryUrl, "repo-url", "", "e.g. https://<token>@github.com/<org>/<repo>. If repo-path is provided, this is ignored.")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&slackWebhookUrl, "slack-url", "", "Slack webhook URL")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of concurrent projects to check. Defaults to 4.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level. Options: trace, debug, info, warn, error, fatal, panic")
	flag.Parse()

	validateArgs(repositoryUrl, repositoryPath, slackWebhookUrl, branch)

	zerolog.SetGlobalLevel(utils.ParseLogLevel(logLevel))

	repoDir, shouldDelete := determineRepositoryDir(repositoryUrl, repositoryPath, branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after the program finishes")
		defer os.RemoveAll(repoDir)
	}

	driftDetector := drift.NewDriftDetector(repoDir, concurrency)
	analysisResult := driftDetector.DetectDrift()

	if analysisResult.TotalDrifted > 0 {
		log.Info().Msgf("Drifted projects: %d", analysisResult.TotalDrifted)
		log.Info().Msg("Sending notification to slack...")
		slack := notification.Slack{Url: slackWebhookUrl}
		slack.Send(analysisResult)
		if slackWebhookUrl != "" {
			log.Info().Msg("Sending notification to slack...")
			slack := notification.Slack{Url: slackWebhookUrl}
			err := slack.Send(analysisResult)
			if err != nil {
				log.Error().Msgf("Failed to send slack notification. %v", err)
			}
		}
	} else {
		log.Info().Msg("No drifts detected")
	}

	if analysisResult.TotalDrifted > 0 {
		os.Exit(1)
	}
}
