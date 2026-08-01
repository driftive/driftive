package summary

import (
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	_ "embed"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

//go:embed tests/expected_summary.md
var expected string

//go:embed tests/expected_summary_clean.md
var expectedClean string

const stateBlockStart = "<!--\nsummary-state-start"

var analysisTime = time.Date(2026, 7, 31, 14, 2, 33, 0, time.UTC)

// visibleBody drops the hidden state block so the goldens cover only what a reader sees.
func visibleBody(t *testing.T, body string) string {
	t.Helper()
	idx := strings.Index(body, stateBlockStart)
	if idx == -1 {
		t.Fatalf("body is missing the state block:\n%s", body)
	}
	return strings.Trim(body[:idx], " \n")
}

func projectResult(dir string, drifted, succeeded, skipped bool, phase string) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project:        models.TypedProject{Dir: dir, Type: models.Terraform},
		Drifted:        drifted,
		Succeeded:      succeeded,
		SkippedDueToPR: skipped,
		FailedPhase:    phase,
	}
}

func projectIssue(dir string, number int, kind string) types.ProjectIssue {
	return types.ProjectIssue{
		Project: models.Project{Dir: dir},
		Issue:   vcstypes.VCSIssue{Number: number},
		Kind:    kind,
	}
}

// fullRun is the fixture behind tests/expected_summary.md.
func fullRun() (drift.DriftDetectionResult, *types.GithubState) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			projectResult("infra/prod/vpc", true, true, false, ""),
			projectResult("infra/prod/rds", true, true, false, ""),
			projectResult("infra/stg/eks", true, true, false, ""),
			projectResult("infra/prod/iam", false, false, false, drift.PhasePlan),
			projectResult("modules/net", false, false, false, drift.PhaseInit),
			projectResult("infra/dev/vpc", true, true, true, ""),
			projectResult("infra/dev/rds", false, true, false, ""),
			projectResult("modules/vpc", false, true, false, ""),
		},
		TotalProjects: 8,
		Duration:      4*time.Minute + 12*time.Second + 341*time.Millisecond,
	}

	state := &types.GithubState{
		DriftIssuesOpen: []types.ProjectIssue{
			projectIssue("infra/prod/vpc", 128, types.DriftIssueKind),
			projectIssue("infra/prod/rds", 131, types.DriftIssueKind),
			projectIssue("infra/legacy/dns", 97, types.DriftIssueKind),
		},
		ErrorIssuesOpen: []types.ProjectIssue{
			projectIssue("infra/prod/iam", 132, types.ErrorIssueKind),
		},
		RateLimitedDrifts: []string{"infra/stg/eks"},
	}
	return result, state
}

func TestGetSummaryIssueBody(t *testing.T) {
	result, state := fullRun()
	summary := buildSummary(result, state, "https://app.driftive.cloud/gh/acme/infra/run/42", analysisTime)

	body, err := getSummaryIssueBody(summary)
	if err != nil {
		t.Fatalf("getSummaryIssueBody() error = %v", err)
	}

	got := visibleBody(t, *body)
	want := strings.Trim(expected, " \n")
	if got != want {
		t.Errorf("summary body mismatch\n--- want ---\n%s\n--- got ---\n%s\n---", want, got)
	}
}

func TestGetSummaryIssueBodyCleanRun(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			projectResult("infra/a", false, true, false, ""),
			projectResult("infra/b", false, true, false, ""),
			projectResult("infra/c", false, true, false, ""),
		},
		TotalProjects: 3,
		Duration:      30 * time.Second,
	}

	summary := buildSummary(result, &types.GithubState{}, "", analysisTime)
	body, err := getSummaryIssueBody(summary)
	if err != nil {
		t.Fatalf("getSummaryIssueBody() error = %v", err)
	}

	got := visibleBody(t, *body)
	want := strings.Trim(expectedClean, " \n")
	if got != want {
		t.Errorf("clean summary body mismatch\n--- want ---\n%s\n--- got ---\n%s\n---", want, got)
	}
}

func TestSummaryPreservesStateBlock(t *testing.T) {
	result, state := fullRun()
	summary := buildSummary(result, state, "", analysisTime)

	body, err := getSummaryIssueBody(summary)
	if err != nil {
		t.Fatalf("getSummaryIssueBody() error = %v", err)
	}

	start := strings.Index(*body, "summary-state-start")
	end := strings.Index(*body, "summary-state-end")
	if start == -1 || end == -1 || end < start {
		t.Fatalf("state block markers missing or out of order:\n%s", *body)
	}

	raw := strings.TrimSpace((*body)[start+len("summary-state-start") : end])
	var roundTripped GithubSummary
	if err := json.Unmarshal([]byte(raw), &roundTripped); err != nil {
		t.Fatalf("state block does not round-trip: %v\nraw: %s", err, raw)
	}
	if roundTripped.NumDrifted != 3 || roundTripped.NumErrored != 2 {
		t.Errorf("round-tripped counts = drifted %d errored %d, want 3 and 2",
			roundTripped.NumDrifted, roundTripped.NumErrored)
	}
	if roundTripped.LastAnalysisDate != analysisTime.Format(time.RFC3339) {
		t.Errorf("LastAnalysisDate = %q", roundTripped.LastAnalysisDate)
	}
}

func TestBuildSummaryJoinsIssueNumbersByDir(t *testing.T) {
	result, state := fullRun()
	summary := buildSummary(result, state, "", analysisTime)

	byDir := map[string]int{}
	for _, p := range summary.Drifted {
		byDir[p.Dir] = p.IssueNumber
	}
	if byDir["infra/prod/vpc"] != 128 || byDir["infra/prod/rds"] != 131 {
		t.Errorf("drift issue numbers not joined: %v", byDir)
	}
	if summary.Errored[0].Dir != "infra/prod/iam" || summary.Errored[0].IssueNumber != 132 {
		t.Errorf("error issue number not joined: %+v", summary.Errored[0])
	}
}

func TestBuildSummaryMarksRateLimited(t *testing.T) {
	result, state := fullRun()
	state.RateLimitedErrors = []string{"modules/net"}

	summary := buildSummary(result, state, "", analysisTime)

	var eks, net SummaryProject
	for _, p := range summary.Drifted {
		if p.Dir == "infra/stg/eks" {
			eks = p
		}
	}
	for _, p := range summary.Errored {
		if p.Dir == "modules/net" {
			net = p
		}
	}

	if !eks.RateLimited {
		t.Error("expected infra/stg/eks to be marked rate limited")
	}
	if !net.RateLimited {
		t.Error("expected modules/net to be marked rate limited")
	}
	if !summary.HasRateLimited() {
		t.Error("HasRateLimited() = false")
	}
}

func TestBuildSummaryCountsFromResultsNotCounters(t *testing.T) {
	result, state := fullRun()
	result.TotalDrifted = 99
	result.TotalErrored = 99
	result.TotalSkipped = 99

	summary := buildSummary(result, state, "", analysisTime)

	if summary.NumDrifted != 3 || summary.NumErrored != 2 || summary.NumSkipped != 1 || summary.NumClean != 2 {
		t.Errorf("counts = drifted %d errored %d skipped %d clean %d, want 3/2/1/2",
			summary.NumDrifted, summary.NumErrored, summary.NumSkipped, summary.NumClean)
	}
}

func TestBuildSummaryFailedPhaseColumn(t *testing.T) {
	result, state := fullRun()
	summary := buildSummary(result, state, "", analysisTime)

	phases := map[string]string{}
	for _, p := range summary.Errored {
		phases[p.Dir] = p.FailedPhase
	}
	if phases["infra/prod/iam"] != drift.PhasePlan {
		t.Errorf("infra/prod/iam phase = %q, want plan", phases["infra/prod/iam"])
	}
	if phases["modules/net"] != drift.PhaseInit {
		t.Errorf("modules/net phase = %q, want init", phases["modules/net"])
	}
}

func TestBuildSummaryMovesUnmatchedIssuesToOtherIssues(t *testing.T) {
	t.Run("project absent from the run", func(t *testing.T) {
		result, state := fullRun()
		summary := buildSummary(result, state, "", analysisTime)

		dirs := make([]string, 0, len(summary.OtherIssues))
		for _, p := range summary.OtherIssues {
			dirs = append(dirs, p.Dir)
		}
		if !slices.Equal(dirs, []string{"infra/legacy/dns"}) {
			t.Errorf("OtherIssues = %v, want [infra/legacy/dns]", dirs)
		}
	})

	t.Run("clean project with close_resolved off", func(t *testing.T) {
		result := drift.DriftDetectionResult{
			ProjectResults: []drift.DriftProjectResult{projectResult("infra/a", false, true, false, "")},
			TotalProjects:  1,
		}
		state := &types.GithubState{
			DriftIssuesOpen: []types.ProjectIssue{projectIssue("infra/a", 5, types.DriftIssueKind)},
		}

		summary := buildSummary(result, state, "", analysisTime)

		if len(summary.OtherIssues) != 1 || summary.OtherIssues[0].IssueNumber != 5 {
			t.Errorf("OtherIssues = %+v, want the still-open issue for the now-clean project", summary.OtherIssues)
		}
	})
}

func TestBuildSummaryNotCheckedOnCancelledRun(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{projectResult("infra/a", false, true, false, "")},
		TotalProjects:  4,
	}

	summary := buildSummary(result, &types.GithubState{}, "", analysisTime)

	if summary.NumNotChecked != 3 {
		t.Errorf("NumNotChecked = %d, want 3", summary.NumNotChecked)
	}
}

func TestBuildSummaryNilStateIsSafe(t *testing.T) {
	result, _ := fullRun()

	summary := buildSummary(result, nil, "", analysisTime)

	if summary.NumDrifted != 3 {
		t.Errorf("NumDrifted = %d, want 3", summary.NumDrifted)
	}
	for _, p := range summary.Drifted {
		if p.IssueNumber != 0 {
			t.Errorf("expected no issue numbers without state, got %+v", p)
		}
	}
}

func TestDirCellEscapesPipe(t *testing.T) {
	p := SummaryProject{Dir: "infra/we|rd"}

	if got := p.DirCell(); got != "`infra/we\\|rd`" {
		t.Errorf("DirCell() = %q, want the pipe escaped", got)
	}
}

func TestIssueLink(t *testing.T) {
	tests := []struct {
		name string
		p    SummaryProject
		want string
	}{
		{"with issue", SummaryProject{IssueNumber: 128}, "[#128](../issues/128)"},
		{"rate limited", SummaryProject{RateLimited: true}, "— _rate limited_"},
		{"no issue", SummaryProject{}, "—"},
		{"issue wins over rate limited", SummaryProject{IssueNumber: 7, RateLimited: true}, "[#7](../issues/7)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.IssueLink(); got != tt.want {
				t.Errorf("IssueLink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSummaryRoundsDuration(t *testing.T) {
	result, state := fullRun()
	summary := buildSummary(result, state, "", analysisTime)

	if summary.Duration != "4m12s" {
		t.Errorf("Duration = %q, want 4m12s", summary.Duration)
	}
}
