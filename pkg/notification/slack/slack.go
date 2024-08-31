package slack

import (
	"bytes"
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models/backend"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

type Slack struct {
	Url         string
	IssuesState *backend.DriftIssuesState
}

func (slack Slack) Send(ctx context.Context, driftResult drift.DriftDetectionResult) error {
	if driftResult.TotalDrifted == 0 && !didResolveIssues(slack.IssuesState) {
		log.Info().Msg("No drift detected. Skipping slack notification")
		return nil
	}

	httpClient := &http.Client{}
	message := ":bangbang: State Drift detected in projects\n"
	message += fmt.Sprintf(":gear: Drifts `%d`/`%d`\n", driftResult.TotalDrifted, driftResult.TotalProjects)
	message += fmt.Sprintf(":clock1: Analysis duration `%s`\n", driftResult.Duration.String())

	if slack.IssuesState != nil && slack.IssuesState.StateUpdated && slack.IssuesState.NumResolvedIssues > 0 {
		message += fmt.Sprintf(":white_check_mark: Resolved issues since last analysis `%d`\n", slack.IssuesState.NumResolvedIssues)
	}

	if driftResult.TotalDrifted > 0 {
		message += ":point_down: Projects with state drifts \n\n```"
		for _, project := range driftResult.ProjectResults {
			if project.Drifted {
				message += fmt.Sprintf("%s\n", project.Project.Dir)
			}
		}
		message += "```"
	}

	type SlackMessage struct {
		Text string `json:"text"`
	}
	slackMessage := SlackMessage{
		Text: message,
	}
	jsonData, err := json.Marshal(slackMessage)
	if err != nil {
		log.Error().Msgf("failed to marshal slack message. %v", err)
		return fmt.Errorf("failed to marshal slack message. %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", slack.Url, bytes.NewBuffer(jsonData))
	if err != nil {
		msg := fmt.Sprintf("failed to create slack request. %v", err)
		log.Error().Msg(msg)
		return errors.New(msg)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		msg := fmt.Sprintf("failed to send slack message. %v", err)
		log.Error().Msg(msg)
		return errors.New(msg)
	}
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read response body. %v", err)
			log.Error().Msg(msg)
			return errors.New(msg)
		}
		msg := fmt.Sprintf("failed to send slack request. %v. Body: %v", resp.Status, body)
		log.Error().Msg(msg)
		return errors.New(msg)
	}
	resp.Body.Close()

	return nil
}

func didResolveIssues(state *backend.DriftIssuesState) bool {
	return state != nil && state.StateUpdated && state.NumResolvedIssues > 0
}
