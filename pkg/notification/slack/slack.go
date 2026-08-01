package slack

import (
	"bytes"
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models/backend"
	"driftive/pkg/notification/report"
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
	colorWarning = "#ED8936" // Orange for errors without drift
	colorSuccess = "#38A169" // Green for all resolved
)

// maxProjectListChars is the safe character budget for one project list section. Slack's limit
// is 3000 chars per section text field; the rest is headroom for the heading and the truncation
// suffix. The drifted and errored lists are separate sections and each gets the full budget.
const maxProjectListChars = 2800

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
	Color    string       `json:"color"`
	Blocks   []slackBlock `json:"blocks"`
	Fallback string       `json:"fallback,omitempty"`
}

type slackMessage struct {
	Text        string            `json:"text,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type Slack struct {
	Url          string
	IssuesState  *backend.DriftIssuesState
	DashboardURL string
	// Repo is "owner/name" from the GitHub Actions context, used to identify the source
	// repository and to build issue links. Empty outside GitHub Actions.
	Repo string
	// DriftIssues and ErrorIssues map a project dir to its open GitHub issue number. Nil when
	// GitHub issues are disabled, in which case rows render as plain text.
	DriftIssues map[string]int
	ErrorIssues map[string]int
}

// projectLine is one entry in a Slack project list.
type projectLine struct {
	Dir string
	// URL links the dir to its GitHub issue. Empty renders the dir as plain code text.
	URL string
	// Note is an optional trailing parenthetical, such as the phase that failed.
	Note string
}

func (slack Slack) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) error {
	summary := report.Classify(driftResult)

	if !summary.HasFindings() && !didResolveIssues(slack.IssuesState) {
		log.Info().Msg("No drifts or errors detected. Skipping slack notification")
		return nil
	}

	message := slack.buildBlockKitMessage(summary)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error().Msgf("failed to marshal slack message. %v", err)
		return fmt.Errorf("failed to marshal slack message. %w", err)
	}

	return slack.sendMessage(ctx, jsonData)
}

func (slack Slack) buildBlockKitMessage(summary report.Summary) slackMessage {
	var blocks []slackBlock

	color, headerText := slack.headline(summary)

	blocks = append(blocks, slackBlock{
		Type: "header",
		Text: &slackTextObject{Type: "plain_text", Text: headerText},
	})

	blocks = append(blocks, slackBlock{Type: "section", Fields: slack.statsFields(summary)})

	if didResolveIssues(slack.IssuesState) {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObject{Type: "mrkdwn", Text: resolvedText(slack.IssuesState)},
		})
	}

	if summary.HasFindings() {
		blocks = append(blocks, slackBlock{Type: "divider"})
	}

	if summary.NumDrifted() > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObject{
				Type: "mrkdwn",
				Text: slack.renderProjectList("*Drifted Projects:*", slack.driftedLines(summary), maxProjectListChars),
			},
		})
	}

	if summary.NumErrored() > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObject{
				Type: "mrkdwn",
				Text: slack.renderProjectList("*Failed Projects:*", slack.erroredLines(summary), maxProjectListChars),
			},
		})
	}

	if slack.DashboardURL != "" {
		blocks = append(blocks, slackBlock{
			Type: "actions",
			Elements: []any{
				slackButtonElement{
					Type: "button",
					Text: slackTextObject{Type: "plain_text", Text: "View in Dashboard"},
					URL:  slack.DashboardURL,
				},
			},
		})
	}

	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []any{
			slackTextObject{Type: "mrkdwn", Text: slack.contextText()},
		},
	})

	return slackMessage{
		Attachments: []slackAttachment{
			{
				Color:    color,
				Blocks:   blocks,
				Fallback: slack.fallbackText(summary),
			},
		},
	}
}

func (slack Slack) headline(summary report.Summary) (color string, header string) {
	switch {
	case summary.NumDrifted() > 0:
		return colorDanger, ":warning: Drift Detected"
	case summary.NumErrored() > 0:
		return colorWarning, ":rotating_light: Analysis Errors"
	case didResolveIssues(slack.IssuesState):
		return colorSuccess, ":white_check_mark: All Drifts Resolved"
	}
	return "", ""
}

// statsFields emits at most 4 fields, which Slack lays out two per row. Duration is always
// last so the grid keeps a stable shape.
func (slack Slack) statsFields(summary report.Summary) []slackTextObject {
	fields := []slackTextObject{{
		Type: "mrkdwn",
		Text: fmt.Sprintf("*Drifted*\n%d / %d projects", summary.NumDrifted(), summary.TotalProjects),
	}}

	if summary.NumErrored() > 0 {
		fields = append(fields, slackTextObject{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Errored*\n%d", summary.NumErrored()),
		})
	}
	if summary.NumSkipped() > 0 {
		fields = append(fields, slackTextObject{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Skipped*\n%d (open PR)", summary.NumSkipped()),
		})
	}

	return append(fields, slackTextObject{
		Type: "mrkdwn",
		Text: fmt.Sprintf("*Duration*\n%s", summary.DurationText()),
	})
}

func (slack Slack) driftedLines(summary report.Summary) []projectLine {
	lines := make([]projectLine, 0, summary.NumDrifted())
	for _, p := range summary.Drifted {
		lines = append(lines, projectLine{Dir: p.Dir, URL: slack.issueURL(slack.DriftIssues, p.Dir)})
	}
	return lines
}

func (slack Slack) erroredLines(summary report.Summary) []projectLine {
	lines := make([]projectLine, 0, summary.NumErrored())
	for _, p := range summary.Errored {
		line := projectLine{Dir: p.Dir, URL: slack.issueURL(slack.ErrorIssues, p.Dir)}
		if p.FailedPhase != "" {
			line.Note = p.FailedPhase
		}
		lines = append(lines, line)
	}
	return lines
}

func (slack Slack) issueURL(issues map[string]int, dir string) string {
	if slack.Repo == "" {
		return ""
	}
	number, ok := issues[dir]
	if !ok || number <= 0 {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", slack.Repo, number)
}

// renderProjectList builds a section's text: a bold heading followed by one bullet per project,
// stopping before budget bytes and appending a "…and N more" suffix.
func (slack Slack) renderProjectList(heading string, lines []projectLine, budget int) string {
	var out strings.Builder
	out.WriteString(heading + "\n")

	for i, line := range lines {
		rendered := line.render()
		suffix := slack.buildTruncationSuffix(len(lines) - i)
		if out.Len()+len(rendered)+len(suffix) > budget {
			out.WriteString(suffix)
			break
		}
		out.WriteString(rendered)
	}

	return out.String()
}

func (line projectLine) render() string {
	label := "`" + line.Dir + "`"
	if line.URL != "" {
		label = fmt.Sprintf("<%s|%s>", line.URL, line.Dir)
	}
	if line.Note != "" {
		return fmt.Sprintf("• %s _(%s)_\n", label, line.Note)
	}
	return fmt.Sprintf("• %s\n", label)
}

func (slack Slack) contextText() string {
	if slack.Repo == "" {
		return "Detected by Driftive"
	}
	return fmt.Sprintf("<https://github.com/%s|%s> · Detected by Driftive", slack.Repo, slack.Repo)
}

// fallbackText is what push notifications and Block Kit-less clients show, so it carries the
// repository as well as the counts.
func (slack Slack) fallbackText(summary report.Summary) string {
	var text string
	switch {
	case summary.NumDrifted() > 0 && summary.NumErrored() > 0:
		text = fmt.Sprintf("Drift detected in %d project(s), %d failed to analyze",
			summary.NumDrifted(), summary.NumErrored())
	case summary.NumDrifted() > 0:
		text = fmt.Sprintf("Drift detected in %d project(s)", summary.NumDrifted())
	case summary.NumErrored() > 0:
		text = fmt.Sprintf("%d project(s) failed to analyze", summary.NumErrored())
	default:
		text = "All drifts resolved"
	}

	if slack.Repo == "" {
		return text
	}
	return fmt.Sprintf("[%s] %s", slack.Repo, text)
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

func didResolveIssues(state *backend.DriftIssuesState) bool {
	return state != nil && state.StateUpdated &&
		(state.NumResolvedIssues > 0 || state.NumResolvedErrorIssues > 0)
}

func resolvedText(state *backend.DriftIssuesState) string {
	drifts, errored := state.NumResolvedIssues, state.NumResolvedErrorIssues
	switch {
	case drifts > 0 && errored > 0:
		return fmt.Sprintf(":tada: *%d issue(s)* and *%d error issue(s) resolved* since last analysis", drifts, errored)
	case errored > 0:
		return fmt.Sprintf(":tada: *%d error issue(s) resolved* since last analysis", errored)
	}
	return fmt.Sprintf(":tada: *%d issue(s) resolved* since last analysis", drifts)
}

func (slack Slack) buildTruncationSuffix(remaining int) string {
	if remaining <= 0 {
		return ""
	}
	if slack.DashboardURL != "" {
		return fmt.Sprintf("_...and %d more project(s). View all in the dashboard._\n", remaining)
	}
	return fmt.Sprintf("_...and %d more project(s)_\n", remaining)
}
