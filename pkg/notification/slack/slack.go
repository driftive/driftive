package slack

import (
	"bytes"
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models/backend"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// Block Kit color constants
const (
	colorDanger  = "#E53E3E" // Red for drifts detected
	colorWarning = "#ED8936" // Orange for warnings
	colorSuccess = "#38A169" // Green for all resolved
)

// Slack Block Kit types
type slackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackButtonElement struct {
	Type string          `json:"type"`
	Text slackTextObject `json:"text"`
	URL  string          `json:"url,omitempty"`
}

type slackBlock struct {
	Type     string            `json:"type"`
	Text     *slackTextObject  `json:"text,omitempty"`
	Fields   []slackTextObject `json:"fields,omitempty"`
	Elements []any             `json:"elements,omitempty"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}

type slackMessage struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type Slack struct {
	Url          string
	IssuesState  *backend.DriftIssuesState
	DashboardURL string
}

func (slack Slack) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) error {
	nonSkippedDrifts := countNonSkippedDrifts(driftResult)

	if nonSkippedDrifts == 0 && !didResolveIssues(slack.IssuesState) {
		log.Info().Msg("No drift detected. Skipping slack notification")
		return nil
	}

	message := slack.buildBlockKitMessage(driftResult, nonSkippedDrifts)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error().Msgf("failed to marshal slack message. %v", err)
		return fmt.Errorf("failed to marshal slack message. %w", err)
	}

	return slack.sendMessage(ctx, jsonData)
}

func (slack Slack) buildBlockKitMessage(driftResult drift.DriftDetectionResult, nonSkippedDrifts int) slackMessage {
	var blocks []slackBlock
	var color string
	var headerText string

	// Determine header and color based on state
	if nonSkippedDrifts > 0 {
		color = colorDanger
		headerText = ":warning: Infrastructure Drift Detected"
	} else if didResolveIssues(slack.IssuesState) {
		color = colorSuccess
		headerText = ":white_check_mark: All Drifts Resolved"
	}

	// Header block
	blocks = append(blocks, slackBlock{
		Type: "header",
		Text: &slackTextObject{
			Type: "plain_text",
			Text: headerText,
		},
	})

	// Stats section with fields
	statsFields := []slackTextObject{
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Drifts*\n%d / %d projects", nonSkippedDrifts, driftResult.TotalProjects),
		},
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Duration*\n%s", driftResult.Duration.String()),
		},
	}

	blocks = append(blocks, slackBlock{
		Type:   "section",
		Fields: statsFields,
	})

	// Resolved issues section (if any)
	if didResolveIssues(slack.IssuesState) {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf(":tada: *%d issue(s) resolved* since last analysis", slack.IssuesState.NumResolvedIssues),
			},
		})
	}

	// Drifted projects list
	if nonSkippedDrifts > 0 {
		blocks = append(blocks, slackBlock{
			Type: "divider",
		})

		// Build project list
		var projectList strings.Builder
		projectList.WriteString("*Affected Projects:*\n")

		for _, project := range driftResult.ProjectResults {
			if project.Drifted && !project.SkippedDueToPR {
				projectList.WriteString(fmt.Sprintf("• `%s`\n", project.Project.Dir))
			}
		}

		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObject{
				Type: "mrkdwn",
				Text: projectList.String(),
			},
		})
	}

	// Dashboard button (if URL is available)
	if slack.DashboardURL != "" {
		blocks = append(blocks, slackBlock{
			Type: "actions",
			Elements: []any{
				slackButtonElement{
					Type: "button",
					Text: slackTextObject{
						Type: "plain_text",
						Text: "View in Dashboard",
					},
					URL: slack.DashboardURL,
				},
			},
		})
	}

	// Context footer
	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []any{
			slackTextObject{
				Type: "mrkdwn",
				Text: "Detected by Driftive",
			},
		},
	})

	return slackMessage{
		Text: headerText, // Fallback text for notifications
		Attachments: []slackAttachment{
			{
				Color:  color,
				Blocks: blocks,
			},
		},
	}
}

func (slack Slack) sendMessage(ctx context.Context, jsonData []byte) error {
	httpClient := &http.Client{}

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read response body. %v", err)
			log.Error().Msg(msg)
			return errors.New(msg)
		}
		msg := fmt.Sprintf("failed to send slack request. %v. Body: %s", resp.Status, string(body))
		log.Error().Msg(msg)
		return errors.New(msg)
	}

	return nil
}

func countNonSkippedDrifts(driftResult drift.DriftDetectionResult) int {
	count := 0
	for _, project := range driftResult.ProjectResults {
		if project.Drifted && !project.SkippedDueToPR {
			count++
		}
	}
	return count
}

func didResolveIssues(state *backend.DriftIssuesState) bool {
	return state != nil && state.StateUpdated && state.NumResolvedIssues > 0
}
