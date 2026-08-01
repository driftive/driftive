package github

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	"slices"
	"testing"
)

func rateLimitedMock() *mockVCS {
	return &mockVCS{
		resultFor: func(_ types.GithubIssue) vcstypes.CreateOrUpdateResult {
			return vcstypes.CreateOrUpdateResult{Created: false, RateLimited: true}
		},
	}
}

// TestErrorIssueRateLimitRecorded pins the dropped signal: max_open_issues applies to error
// issues, but the RateLimited result was only ever checked on the drift path.
func TestErrorIssueRateLimitRecorded(t *testing.T) {
	mock := rateLimitedMock()
	n := newNotification(mock, true, true)

	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{{
			Project:     models.TypedProject{Dir: "infra/prod"},
			Succeeded:   false,
			FailedPhase: drift.PhasePlan,
			PlanOutput:  "Planning failed.",
		}},
		TotalProjects: 1,
	}

	state, err := n.HandleIssues(context.Background(), driftResult, nil)
	if err != nil {
		t.Fatalf("HandleIssues() error = %v", err)
	}

	if !slices.Equal(state.RateLimitedErrors, []string{"infra/prod"}) {
		t.Errorf("RateLimitedErrors = %v, want [infra/prod]", state.RateLimitedErrors)
	}
}

func TestDriftIssueRateLimitRecorded(t *testing.T) {
	mock := rateLimitedMock()
	n := newNotification(mock, true, true)

	driftResult := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{{
			Project:   models.TypedProject{Dir: "infra/prod"},
			Drifted:   true,
			Succeeded: true,
		}},
		TotalProjects: 1,
	}

	state, err := n.HandleIssues(context.Background(), driftResult, nil)
	if err != nil {
		t.Fatalf("HandleIssues() error = %v", err)
	}

	if !slices.Equal(state.RateLimitedDrifts, []string{"infra/prod"}) {
		t.Errorf("RateLimitedDrifts = %v, want [infra/prod]", state.RateLimitedDrifts)
	}
	if len(state.RateLimitedErrors) != 0 {
		t.Errorf("RateLimitedErrors = %v, want empty", state.RateLimitedErrors)
	}
}

func TestIssueNumbersByDirSplitsByKind(t *testing.T) {
	state := &types.GithubState{
		DriftIssuesOpen: []types.ProjectIssue{
			{Project: models.Project{Dir: "infra/a"}, Issue: vcstypes.VCSIssue{Number: 1}, Kind: types.DriftIssueKind},
			{Project: models.Project{Dir: "infra/b"}, Issue: vcstypes.VCSIssue{Number: 2}, Kind: types.DriftIssueKind},
		},
		ErrorIssuesOpen: []types.ProjectIssue{
			{Project: models.Project{Dir: "infra/c"}, Issue: vcstypes.VCSIssue{Number: 3}, Kind: types.ErrorIssueKind},
		},
	}

	drifts := state.IssueNumbersByDir(types.DriftIssueKind)
	if len(drifts) != 2 || drifts["infra/a"] != 1 || drifts["infra/b"] != 2 {
		t.Errorf("drift issue numbers = %v", drifts)
	}

	errored := state.IssueNumbersByDir(types.ErrorIssueKind)
	if len(errored) != 1 || errored["infra/c"] != 3 {
		t.Errorf("error issue numbers = %v", errored)
	}
}

func TestIssueNumbersByDirNilStateIsSafe(t *testing.T) {
	var state *types.GithubState

	if got := state.IssueNumbersByDir(types.DriftIssueKind); got != nil {
		t.Errorf("IssueNumbersByDir() on nil state = %v, want nil", got)
	}
}
