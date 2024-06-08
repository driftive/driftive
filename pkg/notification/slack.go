package notification

import (
	"bytes"
	"driftive/pkg/drift"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Slack struct {
	Url string
}

func (slack Slack) Send(driftResult drift.DriftDetectionResult) error {
	httpClient := &http.Client{}
	message := fmt.Sprintf(":bangbang: State Drift detected in Terragrunt projects\n")
	message += fmt.Sprintf(":gear: Drifts `%d`/`%d`\n", driftResult.TotalDrifted, driftResult.TotalProjects)
	message += fmt.Sprintf(":clock1: Analysis duration `%s`\n", driftResult.Duration.String())
	message += fmt.Sprintf(":point_down: Projects with state drifts \n\n```")
	for _, project := range driftResult.DriftedProjects {
		message += fmt.Sprintf("%s\n", project.Project)
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
		fmt.Printf("failed to marshal slack message. %v", err)
		return fmt.Errorf("failed to marshal slack message. %v", err)
	}

	req, err := http.NewRequest("POST", slack.Url, bytes.NewBuffer(jsonData))
	if err != nil {
		msg := fmt.Sprintf("failed to create slack request. %v", err)
		println(msg)
		return fmt.Errorf(msg)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		msg := fmt.Sprintf("failed to send slack message. %v", err)
		println(msg)
		return fmt.Errorf(msg)
	}
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read response body. %v", err)
			println(msg)
			return fmt.Errorf(msg)
		}
		msg := fmt.Sprintf("failed to send slack request. %v. Body: %v", resp.Status, body)
		println(msg)
		return fmt.Errorf(msg)
	}
	resp.Body.Close()

	return nil
}
