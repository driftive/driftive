package notification

import (
	"bytes"
	"drifter/pkg"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Slack struct {
	Url string
}

func (slack Slack) Send(driftResult pkg.DriftDetectionResult) error {
	httpClient := &http.Client{}
	message := fmt.Sprintf(":bangbang: Drift detected in Terragrunt projects\n")
	message += fmt.Sprintf("Projects checked: %d/%d\n", driftResult.TotalChecked, driftResult.TotalProjects)
	message += fmt.Sprintf("Drifted: %d\n", driftResult.TotalDrifted)
	message += fmt.Sprintf("Drifted projects: :point_down: \n\n```")
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
