package main

import (
	"driftive/pkg/config"
	"driftive/pkg/drift"
	"driftive/pkg/git"
	"driftive/pkg/notification"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
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

	projectsCfg := &config.ProjectsConfig{}
	fileContent, err := os.ReadFile("testdata/rules/detection_rules.yml")
	if err != nil {
		log.Fatal().Msgf("Failed to read detection rules. %v", err)
	}
	err = yaml.Unmarshal(fileContent, &projectsCfg)
	if err != nil {
		log.Fatal().Msgf("Failed to parse detection rules. %v", err)
	}

	projects := config.GetProjectsByRules(cfg.RepositoryPath, projectsCfg)
	log.Info().Msgf("Projects detected: %d", len(projects))

	repoDir, shouldDelete := determineRepositoryDir(cfg.RepositoryUrl, cfg.RepositoryPath, cfg.Branch)
	if shouldDelete {
		log.Debug().Msg("Temp dir will be deleted after driftive finishes.")
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

func parseOnOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func showInitMessage(cfg config.DriftiveConfig) {
	log.Info().Msg("Starting driftive...")
	log.Info().Msgf("Options: concurrency: %d. github issues: %s. slack: %s",
		cfg.Concurrency,
		parseOnOff(cfg.EnableGithubIssues),
		parseOnOff(cfg.SlackWebhookUrl != ""))

	if cfg.EnableGithubIssues && (cfg.GithubToken == "" || cfg.GithubContext == nil || cfg.GithubContext.Repository == "" || cfg.GithubContext.RepositoryOwner == "") {
		log.Fatal().Msg("Github issues are enabled but the required Github token or context is not provided. " +
			"Use the --github-token flag or set the GITHUB_TOKEN environment variable. " +
			"Also, ensure that the GITHUB_CONTEXT environment variable is set in Github Actions.")
	}
}
