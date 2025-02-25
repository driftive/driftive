package notification

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/models/backend"
	"driftive/pkg/notification/console"
	"driftive/pkg/notification/driftive"
	"driftive/pkg/notification/github"
	"driftive/pkg/notification/slack"
	"github.com/rs/zerolog/log"
)

type NotificationHandler struct {
	repoConfig     *repo.DriftiveRepoConfig
	driftiveConfig *config.DriftiveConfig
}

func NewNotificationHandler(driftiveConfig *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) *NotificationHandler {
	return &NotificationHandler{
		repoConfig:     repoConfig,
		driftiveConfig: driftiveConfig,
	}
}

func (h *NotificationHandler) HandleNotifications(ctx context.Context, analysisResult drift.DriftDetectionResult) {
	issuesState := &backend.DriftIssuesState{
		NumOpenIssues:     -1,
		NumResolvedIssues: -1,
		StateUpdated:      false,
	}
	if h.repoConfig.GitHub.Issues.Enabled && h.driftiveConfig.GithubToken != "" && h.driftiveConfig.GithubContext != nil {
		var err error
		log.Info().Msg("Updating Github issues...")
		gh, err := github.NewGithubIssueNotification(h.driftiveConfig, h.repoConfig)
		if err == nil {
			_, err := gh.Handle(ctx, analysisResult)
			if err != nil {
				log.Error().Err(err).Msg("Failed to update github issues/summary")
			}
		}
	}

	if h.driftiveConfig.EnableStdoutResult {
		stdout := console.NewStdout()
		err := stdout.Handle(ctx, analysisResult)
		if err != nil {
			log.Error().Msgf("Failed to print drifts to stdout. %v", err)
		}
	}

	if h.driftiveConfig.SlackWebhookUrl != "" {
		log.Info().Msg("Sending notification to slack...")
		slackNotification := slack.Slack{Url: h.driftiveConfig.SlackWebhookUrl, IssuesState: issuesState}
		err := slackNotification.Handle(ctx, analysisResult)
		if err != nil {
			log.Error().Msgf("Failed to send slack notification. %v", err)
		}
	}

	if h.driftiveConfig.DriftiveToken != "" && h.driftiveConfig.DriftiveApiUrl != "" {
		log.Info().Msg("Sending notification to driftive api...")
		driftiveApiNotification := driftive.NewDriftiveNotification(h.driftiveConfig.DriftiveApiUrl, h.driftiveConfig.DriftiveToken)
		err := driftiveApiNotification.Handle(ctx, analysisResult)
		if err != nil {
			log.Error().Msgf("Failed to analysis result to driftive api. %v", err)
		}
	}
}
