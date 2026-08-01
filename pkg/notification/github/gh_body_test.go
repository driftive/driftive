package github

import (
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"strings"
	"testing"
	"unicode/utf8"
)

func erroredResult(dir, phase, initOutput, planOutput string) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project:     models.TypedProject{Dir: dir, Type: models.Terraform},
		Drifted:     false,
		Succeeded:   false,
		FailedPhase: phase,
		InitOutput:  initOutput,
		PlanOutput:  planOutput,
	}
}

// TestErrorIssueBodyUsesInitOutputWhenInitFailed pins the bug this change fixes: an init failure
// writes to InitOutput, but the body only ever rendered PlanOutput, so the issue published an
// empty code block.
func TestErrorIssueBodyUsesInitOutputWhenInitFailed(t *testing.T) {
	result := erroredResult("infra/prod", drift.PhaseInit, "Error: Failed to install provider", "")

	body, err := parseGithubBodyTemplate(result, errorIssueBodyTemplate)
	if err != nil {
		t.Fatalf("parseGithubBodyTemplate() error = %v", err)
	}

	if !strings.Contains(*body, "Error: Failed to install provider") {
		t.Errorf("error issue body is missing the init output:\n%s", *body)
	}
}

func TestErrorIssueBodyUsesPlanOutputWhenPlanFailed(t *testing.T) {
	result := erroredResult("infra/prod", drift.PhasePlan, "", "Planning failed. Terraform encountered an error")

	body, err := parseGithubBodyTemplate(result, errorIssueBodyTemplate)
	if err != nil {
		t.Fatalf("parseGithubBodyTemplate() error = %v", err)
	}

	if !strings.Contains(*body, "Planning failed.") {
		t.Errorf("error issue body is missing the plan error output:\n%s", *body)
	}
}

// TestDriftIssueBodyStillUsesPlanOutput guards the drift path: its body must not change, since
// issue updates are decided by an exact body comparison.
func TestDriftIssueBodyStillUsesPlanOutput(t *testing.T) {
	result := drift.DriftProjectResult{
		Project:    models.TypedProject{Dir: "infra/prod", Type: models.Terraform},
		Drifted:    true,
		Succeeded:  true,
		PlanOutput: "Plan: 1 to add, 0 to change, 0 to destroy.",
	}

	body, err := parseGithubBodyTemplate(result, issueBodyTemplate)
	if err != nil {
		t.Fatalf("parseGithubBodyTemplate() error = %v", err)
	}

	if !strings.Contains(*body, "Plan: 1 to add, 0 to change, 0 to destroy.") {
		t.Errorf("drift issue body is missing the plan output:\n%s", *body)
	}
	if !strings.Contains(*body, "State drift in project: `infra/prod`") {
		t.Errorf("drift issue body lost its heading:\n%s", *body)
	}
	if !strings.Contains(*body, `"kind":"drift"`) {
		t.Errorf("drift issue body should carry kind=drift:\n%s", *body)
	}
}

// TestIssueBodyKindIsErrorWhenSucceededFalse covers a drifted-and-failed result, which
// HandleIssues files as an error issue while the body classifier called it a drift.
func TestIssueBodyKindIsErrorWhenSucceededFalse(t *testing.T) {
	result := drift.DriftProjectResult{
		Project:     models.TypedProject{Dir: "infra/prod", Type: models.Terraform},
		Drifted:     true,
		Succeeded:   false,
		FailedPhase: drift.PhasePlan,
		PlanOutput:  "Planning failed.",
	}

	body, err := parseGithubBodyTemplate(result, errorIssueBodyTemplate)
	if err != nil {
		t.Fatalf("parseGithubBodyTemplate() error = %v", err)
	}

	if !strings.Contains(*body, `"kind":"error"`) {
		t.Errorf("a failed project must be tagged kind=error:\n%s", *body)
	}
}

// TestIssueBodyTruncationStaysValidUTF8 covers terraform's diagnostic frame characters, which
// are 3 bytes each — a naive byte slice at the size cap splits one and the GitHub API rejects
// the request.
func TestIssueBodyTruncationStaysValidUTF8(t *testing.T) {
	oversized := strings.Repeat("╷", 70000/3)
	result := erroredResult("infra/prod", drift.PhasePlan, "", oversized)

	body, err := parseGithubBodyTemplate(result, errorIssueBodyTemplate)
	if err != nil {
		t.Fatalf("parseGithubBodyTemplate() error = %v", err)
	}

	if !utf8.ValidString(*body) {
		t.Error("truncated issue body is not valid UTF-8")
	}
	if !strings.Contains(*body, issueBodyProjectNameStartKeyword) {
		t.Error("truncated issue body lost its project metadata marker")
	}
}
