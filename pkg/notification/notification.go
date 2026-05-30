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
	"driftive/pkg/vcs"
	"github.com/rs/zerolog/log"
)

type NotificationHandler struct {
	repoConfig     *repo.DriftiveRepoConfig
	driftiveConfig *config.DriftiveConfig
	vcs            vcs.VCS
}

func NewNotificationHandler(driftiveConfig *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig, vcs vcs.VCS) *NotificationHandler {
	return &NotificationHandler{
		repoConfig:     repoConfig,
		driftiveConfig: driftiveConfig,
		vcs:            vcs,
	}
}

// notifierStatus is the outcome of a single notifier, used to emit a single
// structured summary line at the end of HandleNotifications. Per-failure logs
// are still emitted inline; this is the at-a-glance view for log scrapers.
const (
	notifierOk      = "ok"
	notifierSkipped = "skipped"
	notifierFailed  = "failed"
)

func (h *NotificationHandler) HandleNotifications(ctx context.Context, analysisResult drift.DriftDetectionResult) {
	issuesState := &backend.DriftIssuesState{
		NumOpenIssues:     -1,
		NumResolvedIssues: -1,
		StateUpdated:      false,
	}

	driftiveStatus := notifierSkipped
	githubStatus := notifierSkipped
	stdoutStatus := notifierSkipped
	slackStatus := notifierSkipped

	// Send to Driftive API first to get the dashboard URL for other notifications
	var dashboardURL string
	if h.driftiveConfig.DriftiveToken != "" && h.driftiveConfig.DriftiveApiUrl != "" {
		log.Info().Msg("Sending notification to driftive api...")
		driftiveApiNotification := driftive.NewDriftiveNotification(h.driftiveConfig.DriftiveApiUrl, h.driftiveConfig.DriftiveToken)
		response, err := driftiveApiNotification.Handle(ctx, analysisResult)
		if err != nil {
			driftiveStatus = notifierFailed
			log.Error().Msgf("Failed to send analysis result to driftive api. %v", err)
		} else {
			driftiveStatus = notifierOk
			if response != nil {
				dashboardURL = response.DashboardURL
				log.Info().Msgf("Dashboard URL: %s", dashboardURL)
			}
		}
	}

	if h.repoConfig.GitHub.Issues.Enabled && h.driftiveConfig.GithubToken != "" && h.driftiveConfig.GithubContext != nil {
		log.Info().Msg("Updating Github issues...")
		gh, err := github.NewGithubIssueNotification(h.driftiveConfig, h.repoConfig, h.vcs)
		if err != nil {
			githubStatus = notifierFailed
			log.Error().Err(err).Msg("Failed to construct github issues notifier")
		} else {
			_, err := gh.Handle(ctx, analysisResult)
			if err != nil {
				githubStatus = notifierFailed
				log.Error().Err(err).Msg("Failed to update github issues/summary")
			} else {
				githubStatus = notifierOk
			}
		}
	}

	if h.driftiveConfig.EnableStdoutResult {
		stdout := console.NewStdout()
		err := stdout.Handle(ctx, analysisResult)
		if err != nil {
			stdoutStatus = notifierFailed
			log.Error().Msgf("Failed to print drifts to stdout. %v", err)
		} else {
			stdoutStatus = notifierOk
		}
	}

	if h.driftiveConfig.SlackWebhookUrl != "" {
		log.Info().Msg("Sending notification to slack...")
		slackNotification := slack.Slack{
			Url:          h.driftiveConfig.SlackWebhookUrl,
			IssuesState:  issuesState,
			DashboardURL: dashboardURL,
		}
		err := slackNotification.Handle(ctx, analysisResult)
		if err != nil {
			slackStatus = notifierFailed
			log.Error().Msgf("Failed to send slack notification. %v", err)
		} else {
			slackStatus = notifierOk
		}
	}

	log.Info().
		Str("driftive_api", driftiveStatus).
		Str("github", githubStatus).
		Str("stdout", stdoutStatus).
		Str("slack", slackStatus).
		Msg("notification summary")
}
