package notification

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/models/backend"
	"driftive/pkg/notification/console"
	"driftive/pkg/notification/github"
	"driftive/pkg/notification/slack"
	"github.com/rs/zerolog/log"
)

type NotificationHandler struct {
	repoConfig     *repo.DriftiveRepoConfig
	driftiveConfig *config.DriftiveConfig
}

type NotificationsResult struct {
	Github *github.GithubState
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
			ghResult, err := gh.Send(ctx, analysisResult)
			if err != nil {
				log.Error().Msgf("ErrorIssueKind updating Github issues. %v", err)
			}
			log.Info().Msgf("Github issues updated. Result: %v", ghResult)
		}
	}

	if h.driftiveConfig.EnableStdoutResult {
		stdout := console.NewStdout()
		err := stdout.Send(ctx, analysisResult)
		if err != nil {
			log.Error().Msgf("Failed to print drifts to stdout. %v", err)
		}
	}

	if h.driftiveConfig.SlackWebhookUrl != "" {
		log.Info().Msg("Sending notification to slack...")
		slackNotification := slack.Slack{Url: h.driftiveConfig.SlackWebhookUrl, IssuesState: issuesState}
		err := slackNotification.Send(ctx, analysisResult)
		if err != nil {
			log.Error().Msgf("Failed to send slack notification. %v", err)
		}
	}
}
