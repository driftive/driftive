package notification

import (
	"bytes"
	"context"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

type Slack struct {
	Url        string
	repoConfig *repo.DriftiveRepoConfig
}

func (slack Slack) Send(driftResult drift.DriftDetectionResult) error {
	httpClient := &http.Client{}
	ctx := context.Background()
	message := ":bangbang: State Drift detected in Terragrunt projects\n"
	message += fmt.Sprintf(":gear: Drifts `%d`/`%d`\n", driftResult.TotalDrifted, driftResult.TotalProjects)
	message += fmt.Sprintf(":clock1: Analysis duration `%s`\n", driftResult.Duration.String())
	message += ":point_down: Projects with state drifts \n\n```"
	for _, project := range driftResult.DriftedProjects {
		if project.Drifted {
			message += fmt.Sprintf("%s\n", project.Project.Dir)
		}
	}
	message += "```"

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
		return fmt.Errorf(msg)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		msg := fmt.Sprintf("failed to send slack message. %v", err)
		log.Error().Msg(msg)
		return fmt.Errorf(msg)
	}
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read response body. %v", err)
			log.Error().Msg(msg)
			return fmt.Errorf(msg)
		}
		msg := fmt.Sprintf("failed to send slack request. %v. Body: %v", resp.Status, body)
		log.Error().Msg(msg)
		return fmt.Errorf(msg)
	}
	resp.Body.Close()

	return nil
}
