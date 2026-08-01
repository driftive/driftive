package slack

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/models/backend"
	"driftive/pkg/notification/report"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func drifted(dir string) drift.DriftProjectResult {
	return drift.DriftProjectResult{Project: models.TypedProject{Dir: dir}, Drifted: true, Succeeded: true}
}

func skipped(dir string) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project: models.TypedProject{Dir: dir}, Drifted: true, Succeeded: true, SkippedDueToPR: true,
	}
}

func clean(dir string) drift.DriftProjectResult {
	return drift.DriftProjectResult{Project: models.TypedProject{Dir: dir}, Succeeded: true}
}

func errored(dir, phase string) drift.DriftProjectResult {
	return drift.DriftProjectResult{Project: models.TypedProject{Dir: dir}, FailedPhase: phase}
}

func build(slack Slack, result drift.DriftDetectionResult) slackMessage {
	return slack.buildBlockKitMessage(report.Classify(result))
}

func sectionContaining(message slackMessage, needle string) string {
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "section" && block.Text != nil && strings.Contains(block.Text.Text, needle) {
			return block.Text.Text
		}
	}
	return ""
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
		{
			name:     "only error issues resolved",
			state:    &backend.DriftIssuesState{StateUpdated: true, NumResolvedErrorIssues: 2},
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
			drifted("terraform/production/vpc"),
			drifted("terraform/staging/rds"),
			clean("terraform/dev/s3"),
		},
		TotalProjects: 3,
		Duration:      2*time.Minute + 15*time.Second,
	}

	message := build(slack, driftResult)

	if len(message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(message.Attachments))
	}

	attachment := message.Attachments[0]

	if attachment.Fallback == "" {
		t.Error("expected fallback text to be set on attachment")
	}
	if !strings.Contains(attachment.Fallback, "2 project") {
		t.Errorf("expected fallback to mention project count, got %s", attachment.Fallback)
	}

	if attachment.Color != colorDanger {
		t.Errorf("expected color %s, got %s", colorDanger, attachment.Color)
	}

	if len(attachment.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(attachment.Blocks))
	}

	headerBlock := attachment.Blocks[0]
	if headerBlock.Type != "header" {
		t.Errorf("expected first block to be header, got %s", headerBlock.Type)
	}
	if headerBlock.Text == nil || headerBlock.Text.Text != ":warning: Drift Detected" {
		t.Errorf("expected header to be ':warning: Drift Detected', got '%s'", headerBlock.Text.Text)
	}

	statsBlock := attachment.Blocks[1]
	if statsBlock.Type != "section" {
		t.Errorf("expected second block to be section, got %s", statsBlock.Type)
	}
	if len(statsBlock.Fields) != 2 {
		t.Errorf("expected 2 fields in stats section, got %d", len(statsBlock.Fields))
	}

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

func TestBuildBlockKitMessage_ErrorsOnly(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			errored("terraform/production/iam", drift.PhasePlan),
			errored("modules/net", drift.PhaseInit),
			clean("terraform/dev/s3"),
		},
		TotalProjects: 3,
		Duration:      1 * time.Minute,
	}

	message := build(slack, driftResult)
	attachment := message.Attachments[0]

	if attachment.Color != colorWarning {
		t.Errorf("expected color %s for an errors-only run, got %s", colorWarning, attachment.Color)
	}
	if attachment.Blocks[0].Text.Text != ":rotating_light: Analysis Errors" {
		t.Errorf("header = %q", attachment.Blocks[0].Text.Text)
	}
	if got := sectionContaining(message, "Failed Projects"); got == "" {
		t.Error("expected a Failed Projects section")
	}
	if got := sectionContaining(message, "Drifted Projects"); got != "" {
		t.Error("expected no Drifted Projects section when nothing drifted")
	}
	if !strings.Contains(attachment.Fallback, "2 project(s) failed") {
		t.Errorf("fallback = %q, want it to mention the failures", attachment.Fallback)
	}
}

func TestBuildBlockKitMessage_DriftsAndErrors(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			drifted("infra/prod/vpc"),
			drifted("infra/prod/rds"),
			drifted("infra/stg/eks"),
			errored("infra/prod/iam", drift.PhasePlan),
			errored("modules/net", drift.PhaseInit),
			skipped("infra/dev/vpc"),
		},
		TotalProjects: 6,
		Duration:      4*time.Minute + 12*time.Second,
	}

	message := build(slack, driftResult)
	attachment := message.Attachments[0]

	if attachment.Color != colorDanger {
		t.Errorf("expected red when drift is present, got %s", attachment.Color)
	}
	if attachment.Blocks[0].Text.Text != ":warning: Drift Detected" {
		t.Errorf("header = %q", attachment.Blocks[0].Text.Text)
	}

	fields := attachment.Blocks[1].Fields
	if len(fields) != 4 {
		t.Fatalf("expected 4 stats fields (drifted, errored, skipped, duration), got %d", len(fields))
	}
	if !strings.Contains(fields[0].Text, "Drifted") || !strings.Contains(fields[0].Text, "3 / 6") {
		t.Errorf("first field = %q", fields[0].Text)
	}
	if !strings.Contains(fields[1].Text, "Errored") || !strings.Contains(fields[1].Text, "2") {
		t.Errorf("second field = %q", fields[1].Text)
	}
	if !strings.Contains(fields[2].Text, "Skipped") {
		t.Errorf("third field = %q", fields[2].Text)
	}
	if !strings.Contains(fields[3].Text, "Duration") {
		t.Errorf("last field must be Duration, got %q", fields[3].Text)
	}

	if sectionContaining(message, "Drifted Projects") == "" {
		t.Error("expected a Drifted Projects section")
	}
	if sectionContaining(message, "Failed Projects") == "" {
		t.Error("expected a Failed Projects section")
	}
	if !strings.Contains(attachment.Fallback, "3 project(s)") || !strings.Contains(attachment.Fallback, "2 failed") {
		t.Errorf("fallback = %q, want both counts", attachment.Fallback)
	}
}

func TestBuildBlockKitMessage_ErroredProjectShowsPhase(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			errored("infra/prod/iam", drift.PhasePlan),
			errored("modules/net", drift.PhaseInit),
		},
		TotalProjects: 2,
		Duration:      time.Minute,
	}

	failed := sectionContaining(build(slack, driftResult), "Failed Projects")

	if !strings.Contains(failed, "_(plan)_") {
		t.Errorf("expected the plan phase annotated:\n%s", failed)
	}
	if !strings.Contains(failed, "_(init)_") {
		t.Errorf("expected the init phase annotated:\n%s", failed)
	}
}

func TestBuildBlockKitMessage_ErroredProjectWithoutPhase(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{errored("infra/prod/iam", "")},
		TotalProjects:  1,
		Duration:       time.Minute,
	}

	failed := sectionContaining(build(slack, driftResult), "Failed Projects")

	if strings.Contains(failed, "_()_") {
		t.Errorf("expected no empty phase annotation:\n%s", failed)
	}
	if !strings.Contains(failed, "infra/prod/iam") {
		t.Errorf("expected the project listed:\n%s", failed)
	}
}

func TestBuildBlockKitMessage_LinksToIssuesWhenRepoAndIssueKnown(t *testing.T) {
	slack := Slack{
		Repo:        "acme/infra",
		DriftIssues: map[string]int{"infra/prod/vpc": 128},
		ErrorIssues: map[string]int{"infra/prod/iam": 132},
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			drifted("infra/prod/vpc"),
			drifted("infra/prod/rds"),
			errored("infra/prod/iam", drift.PhasePlan),
		},
		TotalProjects: 3,
		Duration:      time.Minute,
	}

	message := build(slack, driftResult)

	drifts := sectionContaining(message, "Drifted Projects")
	if !strings.Contains(drifts, "<https://github.com/acme/infra/issues/128|infra/prod/vpc>") {
		t.Errorf("expected a linked drift row:\n%s", drifts)
	}
	if !strings.Contains(drifts, "`infra/prod/rds`") {
		t.Errorf("expected the un-issued project as plain code text:\n%s", drifts)
	}

	failed := sectionContaining(message, "Failed Projects")
	if !strings.Contains(failed, "<https://github.com/acme/infra/issues/132|infra/prod/iam>") {
		t.Errorf("expected a linked error row:\n%s", failed)
	}
}

func TestBuildBlockKitMessage_PlainTextWhenRepoUnknown(t *testing.T) {
	slack := Slack{DriftIssues: map[string]int{"infra/prod/vpc": 128}}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("infra/prod/vpc")},
		TotalProjects:  1,
		Duration:       time.Minute,
	}

	drifts := sectionContaining(build(slack, driftResult), "Drifted Projects")

	if strings.Contains(drifts, "github.com") {
		t.Errorf("expected no link without a repo:\n%s", drifts)
	}
	if !strings.Contains(drifts, "`infra/prod/vpc`") {
		t.Errorf("expected plain code text:\n%s", drifts)
	}
}

func TestBuildBlockKitMessage_ContextShowsRepo(t *testing.T) {
	slack := Slack{Repo: "acme/infra"}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("infra/prod/vpc")},
		TotalProjects:  1,
		Duration:       time.Minute,
	}

	message := build(slack, driftResult)

	var contextText string
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "context" {
			if textObj, ok := block.Elements[0].(slackTextObject); ok {
				contextText = textObj.Text
			}
		}
	}

	if !strings.Contains(contextText, "<https://github.com/acme/infra|acme/infra>") {
		t.Errorf("context = %q, want a repo link", contextText)
	}
	if !strings.Contains(contextText, "Driftive") {
		t.Errorf("context = %q, want it to still mention Driftive", contextText)
	}
	if !strings.Contains(message.Attachments[0].Fallback, "[acme/infra]") {
		t.Errorf("fallback = %q, want the repo prefix", message.Attachments[0].Fallback)
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
		ProjectResults: []drift.DriftProjectResult{clean("terraform/production/vpc")},
		TotalProjects:  1,
		Duration:       30 * time.Second,
	}

	message := build(slack, driftResult)

	if len(message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(message.Attachments))
	}
	if message.Attachments[0].Color != colorSuccess {
		t.Errorf("expected color %s, got %s", colorSuccess, message.Attachments[0].Color)
	}

	headerBlock := message.Attachments[0].Blocks[0]
	if !strings.Contains(headerBlock.Text.Text, "Resolved") {
		t.Error("expected header to contain 'Resolved'")
	}

	jsonData, _ := json.Marshal(message)
	if !strings.Contains(string(jsonData), "5 issue(s) resolved") {
		t.Error("expected message to contain resolved issues count")
	}
}

func TestBuildBlockKitMessage_ResolvedWording(t *testing.T) {
	tests := []struct {
		name  string
		state *backend.DriftIssuesState
		want  string
	}{
		{
			name:  "drift issues only",
			state: &backend.DriftIssuesState{StateUpdated: true, NumResolvedIssues: 5},
			want:  ":tada: *5 issue(s) resolved* since last analysis",
		},
		{
			name:  "error issues only",
			state: &backend.DriftIssuesState{StateUpdated: true, NumResolvedErrorIssues: 2},
			want:  ":tada: *2 error issue(s) resolved* since last analysis",
		},
		{
			name:  "both",
			state: &backend.DriftIssuesState{StateUpdated: true, NumResolvedIssues: 5, NumResolvedErrorIssues: 2},
			want:  ":tada: *5 issue(s)* and *2 error issue(s) resolved* since last analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvedText(tt.state); got != tt.want {
				t.Errorf("resolvedText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildBlockKitMessage_SkippedFieldOnlyWhenSkipped(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("terraform/production/vpc")},
		TotalProjects:  1,
		Duration:       time.Minute,
	}

	for _, field := range build(slack, driftResult).Attachments[0].Blocks[1].Fields {
		if strings.Contains(field.Text, "Skipped") {
			t.Errorf("expected no Skipped field when nothing was skipped, got %q", field.Text)
		}
	}
}

func TestBuildBlockKitMessage_WithSkippedProjects(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			drifted("terraform/production/vpc"),
			skipped("terraform/staging/rds"),
		},
		TotalProjects: 2,
		Duration:      1 * time.Minute,
	}

	message := build(slack, driftResult)

	jsonData, _ := json.Marshal(message)
	jsonStr := string(jsonData)

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
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	message := build(slack, driftResult)

	var hasContextBlock bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "context" {
			hasContextBlock = true
			if len(block.Elements) == 0 {
				t.Error("context block should have elements")
			}
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
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	message := build(slack, driftResult)

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

func TestMessageOmitsEmptyTextField(t *testing.T) {
	slack := Slack{}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       time.Minute,
	}

	jsonData, err := json.Marshal(build(slack, driftResult))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, present := raw["text"]; present {
		t.Errorf("expected no empty top-level text field, got %s", jsonData)
	}
}

func TestHandle_SkipsWhenOnlyClean(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not send request when nothing drifted, errored or resolved")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{clean("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandle_SkipsWhenOnlySkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not send request when every drift was skipped")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{skipped("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestHandle_SendsWhenOnlyErrors pins the behavior change: a run where every project failed
// used to be silent.
func TestHandle_SendsWhenOnlyErrors(t *testing.T) {
	var requestReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := Slack{Url: server.URL}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			errored("project1", drift.PhasePlan),
			errored("project2", drift.PhaseInit),
		},
		TotalProjects: 2,
		Duration:      1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !requestReceived {
		t.Error("expected a notification for an all-errors run")
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
		ProjectResults: []drift.DriftProjectResult{drifted("terraform/vpc")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(receivedBody) == 0 {
		t.Error("expected request to be sent")
	}

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
		ProjectResults: []drift.DriftProjectResult{clean("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err != nil {
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
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
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
	slack := Slack{Url: "http://localhost:59999"}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	if err := slack.Handle(context.Background(), driftResult); err == nil {
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
			drifted("terraform/production/vpc"),
			skipped("terraform/staging/rds"),
			clean("terraform/dev/s3"),
		},
		TotalProjects: 3,
		Duration:      2*time.Minute + 15*time.Second,
	}

	message := build(slack, driftResult)

	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Errorf("failed to marshal message to JSON: %v", err)
	}

	var parsed slackMessage
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Errorf("failed to unmarshal JSON back to struct: %v", err)
	}
	if len(parsed.Attachments) != 1 {
		t.Errorf("expected 1 attachment after round-trip, got %d", len(parsed.Attachments))
	}
}

func TestBuildBlockKitMessage_WithDashboardURL(t *testing.T) {
	slack := Slack{
		DashboardURL: "https://driftive.cloud/github/myorg/myrepo/run/abc-123",
	}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	message := build(slack, driftResult)

	var hasActionsBlock bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "actions" {
			hasActionsBlock = true
			if len(block.Elements) == 0 {
				t.Error("actions block should have elements")
			}
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
	slack := Slack{DashboardURL: ""}
	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{drifted("project1")},
		TotalProjects:  1,
		Duration:       1 * time.Minute,
	}

	message := build(slack, driftResult)

	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "actions" {
			t.Error("expected no actions block when dashboard URL is empty")
		}
	}
}

func longDriftResult(n int) drift.DriftDetectionResult {
	var projects []drift.DriftProjectResult
	for i := 0; i < n; i++ {
		projects = append(projects, drifted(fmt.Sprintf("terraform/production/us-east-1/service-name-%03d/sub-module", i)))
	}
	return drift.DriftDetectionResult{ProjectResults: projects, TotalProjects: n, Duration: 2 * time.Minute}
}

func TestBuildBlockKitMessage_TruncatesLongProjectList(t *testing.T) {
	slack := Slack{
		DashboardURL: "https://driftive.cloud/github/myorg/myrepo/run/abc-123",
	}

	message := build(slack, longDriftResult(100))
	projectListText := sectionContaining(message, "Drifted Projects")

	if projectListText == "" {
		t.Fatal("expected to find project list section block")
	}
	if len(projectListText) > 3000 {
		t.Errorf("project list text exceeds 3000 chars: got %d", len(projectListText))
	}
	if !strings.Contains(projectListText, "more project(s)") {
		t.Error("expected truncation message in project list")
	}
	if !strings.Contains(projectListText, "View all in the dashboard") {
		t.Error("expected dashboard reference in truncation message")
	}

	var hasActionsBlock, hasContextBlock bool
	for _, block := range message.Attachments[0].Blocks {
		if block.Type == "actions" {
			hasActionsBlock = true
		}
		if block.Type == "context" {
			hasContextBlock = true
		}
	}
	if !hasActionsBlock {
		t.Error("expected actions block (dashboard button) to still be present")
	}
	if !hasContextBlock {
		t.Error("expected context footer to still be present")
	}
}

func TestBuildBlockKitMessage_TruncationWithoutDashboardURL(t *testing.T) {
	projectListText := sectionContaining(build(Slack{}, longDriftResult(100)), "Drifted Projects")

	if !strings.Contains(projectListText, "more project(s)") {
		t.Error("expected truncation message")
	}
	if strings.Contains(projectListText, "dashboard") {
		t.Error("expected no dashboard reference when DashboardURL is empty")
	}
}

func TestBuildBlockKitMessage_NoTruncationWhenFewProjects(t *testing.T) {
	projects := []drift.DriftProjectResult{
		drifted("terraform/vpc"),
		drifted("terraform/rds"),
		drifted("terraform/s3"),
	}
	driftResult := drift.DriftDetectionResult{ProjectResults: projects, TotalProjects: 3, Duration: time.Minute}

	projectListText := sectionContaining(build(Slack{}, driftResult), "Drifted Projects")

	if strings.Contains(projectListText, "more project(s)") {
		t.Error("expected no truncation for few projects")
	}
	for _, p := range projects {
		if !strings.Contains(projectListText, p.Project.Dir) {
			t.Errorf("expected project %s to be listed", p.Project.Dir)
		}
	}
}

// TestBuildBlockKitMessage_TruncationCountAccuracy counts backticked rows, which is valid
// because this fixture sets no Repo and no issue map, so no row renders as a link.
func TestBuildBlockKitMessage_TruncationCountAccuracy(t *testing.T) {
	totalProjects := 50

	projectListText := sectionContaining(build(Slack{}, longDriftResult(totalProjects)), "Drifted Projects")

	displayed := strings.Count(projectListText, "• `")

	var truncated int
	if strings.Contains(projectListText, "more project(s)") {
		fmt.Sscanf(extractTruncatedCount(projectListText), "%d", &truncated)
	}

	if displayed+truncated != totalProjects {
		t.Errorf("displayed (%d) + truncated (%d) = %d, want %d", displayed, truncated, displayed+truncated, totalProjects)
	}
}

// TestRenderProjectList_IndependentBudgets confirms a long drift list cannot crowd out the
// errors: the two lists are separate section blocks with separate budgets.
func TestRenderProjectList_IndependentBudgets(t *testing.T) {
	result := longDriftResult(200)
	result.ProjectResults = append(result.ProjectResults, errored("modules/net", drift.PhaseInit))
	result.TotalProjects = 201

	message := build(Slack{}, result)

	drifts := sectionContaining(message, "Drifted Projects")
	failed := sectionContaining(message, "Failed Projects")

	if !strings.Contains(drifts, "more project(s)") {
		t.Error("expected the drift list to truncate")
	}
	if !strings.Contains(failed, "modules/net") {
		t.Errorf("errored project must survive a long drift list:\n%s", failed)
	}
	if len(drifts) > 3000 || len(failed) > 3000 {
		t.Errorf("section text over Slack's limit: drifts=%d failed=%d", len(drifts), len(failed))
	}
}

func TestRenderProjectList_HonorsBudget(t *testing.T) {
	lines := make([]projectLine, 0, 100)
	for i := 0; i < 100; i++ {
		lines = append(lines, projectLine{Dir: fmt.Sprintf("terraform/production/service-%03d", i)})
	}

	got := Slack{}.renderProjectList("*Drifted Projects:*", lines, 300)

	if len(got) > 300 {
		t.Errorf("rendered %d chars, budget was 300", len(got))
	}
	if !strings.Contains(got, "more project(s)") {
		t.Errorf("expected a truncation suffix:\n%s", got)
	}
}

func extractTruncatedCount(text string) string {
	idx := strings.Index(text, "...and ")
	if idx == -1 {
		return "0"
	}
	rest := text[idx+len("...and "):]
	spaceIdx := strings.Index(rest, " ")
	if spaceIdx == -1 {
		return "0"
	}
	return rest[:spaceIdx]
}
