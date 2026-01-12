package slack

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/models/backend"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCountNonSkippedDrifts(t *testing.T) {
	tests := []struct {
		name     string
		results  []drift.DriftProjectResult
		expected int
	}{
		{
			name:     "empty results",
			results:  []drift.DriftProjectResult{},
			expected: 0,
		},
		{
			name: "no drifts",
			results: []drift.DriftProjectResult{
				{Project: models.TypedProject{Dir: "project1"}, Drifted: false},
				{Project: models.TypedProject{Dir: "project2"}, Drifted: false},
			},
			expected: 0,
		},
		{
			name: "all drifted",
			results: []drift.DriftProjectResult{
				{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
				{Project: models.TypedProject{Dir: "project2"}, Drifted: true},
			},
			expected: 2,
		},
		{
			name: "some skipped due to PR",
			results: []drift.DriftProjectResult{
				{Project: models.TypedProject{Dir: "project1"}, Drifted: true, SkippedDueToPR: false},
				{Project: models.TypedProject{Dir: "project2"}, Drifted: true, SkippedDueToPR: true},
				{Project: models.TypedProject{Dir: "project3"}, Drifted: true, SkippedDueToPR: false},
			},
			expected: 2,
		},
		{
			name: "all skipped due to PR",
			results: []drift.DriftProjectResult{
				{Project: models.TypedProject{Dir: "project1"}, Drifted: true, SkippedDueToPR: true},
				{Project: models.TypedProject{Dir: "project2"}, Drifted: true, SkippedDueToPR: true},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := drift.DriftDetectionResult{ProjectResults: tt.results}
			got := countNonSkippedDrifts(result)
			if got != tt.expected {
				t.Errorf("countNonSkippedDrifts() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestDidResolveIssues(t *testing.T) {
	tests := []struct {
		name     string
		state    *backend.DriftIssuesState
		expected bool
	}{
		{
			name:     "nil state",
			state:    nil,
			expected: false,
		},
		{
			name:     "state not updated",
			state:    &backend.DriftIssuesState{StateUpdated: false, NumResolvedIssues: 5},
			expected: false,
		},
		{
			name:     "no resolved issues",
			state:    &backend.DriftIssuesState{StateUpdated: true, NumResolvedIssues: 0},
			expected: false,
		},
		{
			name:     "has resolved issues",
			state:    &backend.DriftIssuesState{StateUpdated: true, NumResolvedIssues: 3},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := didResolveIssues(tt.state)
			if got != tt.expected {
				t.Errorf("didResolveIssues() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildBlockKitMessage_WithDrifts(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "terraform/production/vpc"}, Drifted: true},
			{Project: models.TypedProject{Dir: "terraform/staging/rds"}, Drifted: true},
			{Project: models.TypedProject{Dir: "terraform/dev/s3"}, Drifted: false},
		},
		TotalProjects: 3,
		Duration:      2*time.Minute + 15*time.Second,
	}

	message := slack.buildBlockKitMessage(driftResult, 2)

	// Verify basic structure
	if len(message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(message.Attachments))
	}

	attachment := message.Attachments[0]

	// Verify fallback text is set on attachment
	if attachment.Fallback == "" {
		t.Error("expected fallback text to be set on attachment")
	}
	if !strings.Contains(attachment.Fallback, "2 project") {
		t.Errorf("expected fallback to mention project count, got %s", attachment.Fallback)
	}

	// Verify color is danger (red) for drifts
	if attachment.Color != colorDanger {
		t.Errorf("expected color %s, got %s", colorDanger, attachment.Color)
	}

	// Verify blocks exist
	if len(attachment.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(attachment.Blocks))
	}

	// Verify header block
	headerBlock := attachment.Blocks[0]
	if headerBlock.Type != "header" {
		t.Errorf("expected first block to be header, got %s", headerBlock.Type)
	}
	if headerBlock.Text == nil || headerBlock.Text.Text != ":warning: Drift Detected" {
		t.Errorf("expected header to be ':warning: Drift Detected', got '%s'", headerBlock.Text.Text)
	}

	// Verify stats section has fields
	statsBlock := attachment.Blocks[1]
	if statsBlock.Type != "section" {
		t.Errorf("expected second block to be section, got %s", statsBlock.Type)
	}
	if len(statsBlock.Fields) != 2 {
		t.Errorf("expected 2 fields in stats section, got %d", len(statsBlock.Fields))
	}

	// Verify project list contains drifted projects
	jsonData, _ := json.Marshal(message)
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "terraform/production/vpc") {
		t.Error("expected message to contain drifted project 'terraform/production/vpc'")
	}
	if !strings.Contains(jsonStr, "terraform/staging/rds") {
		t.Error("expected message to contain drifted project 'terraform/staging/rds'")
	}
	if strings.Contains(jsonStr, "terraform/dev/s3") {
		t.Error("expected message NOT to contain non-drifted project 'terraform/dev/s3'")
	}
}

func TestBuildBlockKitMessage_AllResolved(t *testing.T) {
	slack := Slack{
		IssuesState: &backend.DriftIssuesState{
			StateUpdated:      true,
			NumResolvedIssues: 5,
		},
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "terraform/production/vpc"}, Drifted: false},
		},
		TotalProjects: 1,
		Duration:      30 * time.Second,
	}

	message := slack.buildBlockKitMessage(driftResult, 0)

	// Verify color is success (green) when all resolved
	if len(message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(message.Attachments))
	}
	if message.Attachments[0].Color != colorSuccess {
		t.Errorf("expected color %s, got %s", colorSuccess, message.Attachments[0].Color)
	}

	// Verify header indicates resolved
	headerBlock := message.Attachments[0].Blocks[0]
	if !strings.Contains(headerBlock.Text.Text, "Resolved") {
		t.Error("expected header to contain 'Resolved'")
	}

	// Verify resolved issues message is present
	jsonData, _ := json.Marshal(message)
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "5 issue(s) resolved") {
		t.Error("expected message to contain resolved issues count")
	}
}

func TestBuildBlockKitMessage_WithSkippedProjects(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "terraform/production/vpc"}, Drifted: true, SkippedDueToPR: false},
			{Project: models.TypedProject{Dir: "terraform/staging/rds"}, Drifted: true, SkippedDueToPR: true},
		},
		TotalProjects: 2,
		Duration:      1 * time.Minute,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	jsonData, _ := json.Marshal(message)
	jsonStr := string(jsonData)

	// Verify only non-skipped project is listed
	if !strings.Contains(jsonStr, "terraform/production/vpc") {
		t.Error("expected message to contain non-skipped drifted project")
	}
	if strings.Contains(jsonStr, "terraform/staging/rds") {
		t.Error("expected message NOT to contain skipped project")
	}
}

func TestBuildBlockKitMessage_HasContextFooter(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	// Find context block
	var hasContextBlock bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "context" {
			hasContextBlock = true
			if len(block.Elements) == 0 {
				t.Error("context block should have elements")
			}
			// Type assert to check the text content
			if textObj, ok := block.Elements[0].(slackTextObject); ok {
				if !strings.Contains(textObj.Text, "Driftive") {
					t.Error("context should mention Driftive")
				}
			}
		}
	}
	if !hasContextBlock {
		t.Error("expected message to have context footer block")
	}
}

func TestBuildBlockKitMessage_HasDividerBeforeProjects(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	// Find divider block
	var hasDivider bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "divider" {
			hasDivider = true
			break
		}
	}
	if !hasDivider {
		t.Error("expected message to have divider block before project list")
	}
}

func TestHandle_SkipsWhenNoDriftsAndNoResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not send request when no drifts and no resolved issues")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: false},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	err := slack.Handle(context.Background(), driftResult)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandle_SendsWhenDriftsDetected(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "terraform/vpc"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	err := slack.Handle(context.Background(), driftResult)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(receivedBody) == 0 {
		t.Error("expected request to be sent")
	}

	// Verify JSON structure
	var message slackMessage
	if err := json.Unmarshal(receivedBody, &message); err != nil {
		t.Errorf("failed to unmarshal sent message: %v", err)
	}

	if len(message.Attachments) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(message.Attachments))
	}
}

func TestHandle_SendsWhenIssuesResolved(t *testing.T) {
	var requestReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{
		Url: server.URL,
		IssuesState: &backend.DriftIssuesState{
			StateUpdated:      true,
			NumResolvedIssues: 2,
		},
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: false},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	err := slack.Handle(context.Background(), driftResult)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !requestReceived {
		t.Error("expected request to be sent when issues are resolved")
	}
}

func TestHandle_ReturnsErrorOnBadStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_token"))
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	err := slack.Handle(context.Background(), driftResult)
	if err == nil {
		t.Error("expected error on bad status code")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error to contain status code, got: %v", err)
	}
}

func TestHandle_ReturnsErrorOnConnectionFailure(t *testing.T) {
	slack := Slack{Url: "http://localhost:59999"} // Non-existent server
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	err := slack.Handle(context.Background(), driftResult)
	if err == nil {
		t.Error("expected error on connection failure")
	}
}

func TestBuildBlockKitMessage_ValidJSON(t *testing.T) {
	slack := Slack{
		IssuesState: &backend.DriftIssuesState{
			StateUpdated:      true,
			NumResolvedIssues: 3,
		},
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "terraform/production/vpc"}, Drifted: true},
			{Project: models.TypedProject{Dir: "terraform/staging/rds"}, Drifted: true, SkippedDueToPR: true},
			{Project: models.TypedProject{Dir: "terraform/dev/s3"}, Drifted: false},
		},
		TotalProjects: 3,
		Duration:      2*time.Minute + 15*time.Second,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	// Ensure it marshals to valid JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Errorf("failed to marshal message to JSON: %v", err)
	}

	// Ensure it can be unmarshalled back
	var parsed slackMessage
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Errorf("failed to unmarshal JSON back to struct: %v", err)
	}

	// Verify structure is preserved
	if len(parsed.Attachments) != 1 {
		t.Errorf("expected 1 attachment after round-trip, got %d", len(parsed.Attachments))
	}
}

func TestBuildBlockKitMessage_WithDashboardURL(t *testing.T) {
	slack := Slack{
		DashboardURL: "https://driftive.cloud/github/myorg/myrepo/run/abc-123",
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	// Find actions block with button
	var hasActionsBlock bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "actions" {
			hasActionsBlock = true
			if len(block.Elements) == 0 {
				t.Error("actions block should have elements")
			}
			// Verify button is present with correct URL
			if btn, ok := block.Elements[0].(slackButtonElement); ok {
				if btn.URL != "https://driftive.cloud/github/myorg/myrepo/run/abc-123" {
					t.Errorf("expected button URL to match dashboard URL, got %s", btn.URL)
				}
				if btn.Text.Text != "View in Dashboard" {
					t.Errorf("expected button text 'View in Dashboard', got %s", btn.Text.Text)
				}
			} else {
				t.Error("expected first element to be a button")
			}
		}
	}
	if !hasActionsBlock {
		t.Error("expected message to have actions block with dashboard button")
	}
}

func TestBuildBlockKitMessage_WithoutDashboardURL(t *testing.T) {
	slack := Slack{
		DashboardURL: "", // No dashboard URL
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{Project: models.TypedProject{Dir: "project1"}, Drifted: true},
		},
		TotalProjects: 1,
		Duration:      1 * time.Minute,
	}

	message := slack.buildBlockKitMessage(driftResult, 1)

	// Verify no actions block exists
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "actions" {
			t.Error("expected no actions block when dashboard URL is empty")
		}
	}
}
