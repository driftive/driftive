package github

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/gh"
	"driftive/pkg/models"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	"testing"
)

// mockVCS records calls to CloseIssue and CreateIssueComment for assertions.
type mockVCS struct {
	openIssues          []*vcstypes.VCSIssue
	closedIssueNumbers  []int
	commentedIssueNums  []int
	createOrUpdateCalls []types.GithubIssue
}

func (m *mockVCS) GetAllOpenRepoIssues(_ context.Context) ([]*vcstypes.VCSIssue, error) {
	return m.openIssues, nil
}

func (m *mockVCS) GetChangedFilesForAllPRs(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockVCS) CreateOrUpdateIssue(_ context.Context, issue types.GithubIssue, _ []*vcstypes.VCSIssue, _ bool) vcstypes.CreateOrUpdateResult {
	m.createOrUpdateCalls = append(m.createOrUpdateCalls, issue)
	return vcstypes.CreateOrUpdateResult{Created: false, RateLimited: false, Issue: nil}
}

func (m *mockVCS) CreateIssueComment(_ context.Context, issueNumber int) error {
	m.commentedIssueNums = append(m.commentedIssueNums, issueNumber)
	return nil
}

func (m *mockVCS) CloseIssue(_ context.Context, issueNumber int) error {
	m.closedIssueNumbers = append(m.closedIssueNumbers, issueNumber)
	return nil
}

func makeIssueBody(dir string, kind string) string {
	return "<!--PROJECT_JSON_START-->{\"project\":{\"dir\":\"" + dir + "\"},\"kind\":\"" + kind + "\"}<!--PROJECT_JSON_END-->"
}

func newNotification(mock *mockVCS, driftCloseResolved, errorCloseResolved bool) *GithubIssueNotification {
	return &GithubIssueNotification{
		config: &config.DriftiveConfig{
			GithubContext: &gh.GithubActionContext{
				Repository:      "owner/repo",
				RepositoryOwner: "owner",
			},
		},
		repoConfig: &repo.DriftiveRepoConfig{
			GitHub: repo.DriftiveRepoConfigGitHub{
				Issues: repo.DriftiveRepoConfigGitHubIssues{
					CloseResolved: driftCloseResolved,
					MaxOpenIssues: 100,
					Errors: repo.DriftiveRepoConfigGitHubIssuesErrors{
						Enabled:       true,
						CloseResolved: errorCloseResolved,
						MaxOpenIssues: 100,
					},
				},
			},
		},
		scm: mock,
	}
}

func TestErroredProjectDoesNotCloseDriftIssue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: false, // errored — drift status unknown
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 0 {
		t.Errorf("expected no issues closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.DriftIssuesResolved) != 0 {
		t.Errorf("expected no drift issues resolved, got %d", len(state.DriftIssuesResolved))
	}
}

func TestResolvedProjectClosesDriftIssue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: true, // resolved
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 1 || mock.closedIssueNumbers[0] != 1 {
		t.Errorf("expected issue 1 closed, got %v", mock.closedIssueNumbers)
	}
	if len(mock.commentedIssueNums) != 1 || mock.commentedIssueNums[0] != 1 {
		t.Errorf("expected comment on issue 1, got %v", mock.commentedIssueNums)
	}
	if len(state.DriftIssuesResolved) != 1 {
		t.Errorf("expected 1 drift issue resolved, got %d", len(state.DriftIssuesResolved))
	}
}

func TestStillDriftedProjectDoesNotCloseIssue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   true,
				Succeeded: true,
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 0 {
		t.Errorf("expected no issues closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.DriftIssuesResolved) != 0 {
		t.Errorf("expected no drift issues resolved, got %d", len(state.DriftIssuesResolved))
	}
}

func TestDriftCloseResolvedFalseErrorCloseResolvedTrue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, false, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
		{Number: 2, Title: "plan error: infra/staging", Body: makeIssueBody("infra/staging", "error")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: true,
			},
			{
				Project:   models.TypedProject{Dir: "infra/staging"},
				Drifted:   false,
				Succeeded: true,
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only error issue should be closed (drift CloseResolved=false)
	if len(mock.closedIssueNumbers) != 1 || mock.closedIssueNumbers[0] != 2 {
		t.Errorf("expected only issue 2 closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.DriftIssuesResolved) != 0 {
		t.Errorf("expected no drift issues resolved, got %d", len(state.DriftIssuesResolved))
	}
	if len(state.ErrorIssuesResolved) != 1 {
		t.Errorf("expected 1 error issue resolved, got %d", len(state.ErrorIssuesResolved))
	}
}

func TestDriftCloseResolvedTrueErrorCloseResolvedFalse(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, false)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
		{Number: 2, Title: "plan error: infra/staging", Body: makeIssueBody("infra/staging", "error")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: true,
			},
			{
				Project:   models.TypedProject{Dir: "infra/staging"},
				Drifted:   false,
				Succeeded: true,
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only drift issue should be closed (error CloseResolved=false)
	if len(mock.closedIssueNumbers) != 1 || mock.closedIssueNumbers[0] != 1 {
		t.Errorf("expected only issue 1 closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.DriftIssuesResolved) != 1 {
		t.Errorf("expected 1 drift issue resolved, got %d", len(state.DriftIssuesResolved))
	}
	if len(state.ErrorIssuesResolved) != 0 {
		t.Errorf("expected no error issues resolved, got %d", len(state.ErrorIssuesResolved))
	}
}

func TestSucceededProjectClosesErrorIssue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 5, Title: "plan error: infra/prod", Body: makeIssueBody("infra/prod", "error")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: true,
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 1 || mock.closedIssueNumbers[0] != 5 {
		t.Errorf("expected issue 5 closed, got %v", mock.closedIssueNumbers)
	}
	if len(mock.commentedIssueNums) != 1 || mock.commentedIssueNums[0] != 5 {
		t.Errorf("expected comment on issue 5, got %v", mock.commentedIssueNums)
	}
	if len(state.ErrorIssuesResolved) != 1 {
		t.Errorf("expected 1 error issue resolved, got %d", len(state.ErrorIssuesResolved))
	}
}

func TestStillErroredProjectDoesNotCloseErrorIssue(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 5, Title: "plan error: infra/prod", Body: makeIssueBody("infra/prod", "error")},
	}

	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/prod"},
				Drifted:   false,
				Succeeded: false, // still erroring
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 0 {
		t.Errorf("expected no issues closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.ErrorIssuesResolved) != 0 {
		t.Errorf("expected no error issues resolved, got %d", len(state.ErrorIssuesResolved))
	}
}

func TestOpenIssueWithNoMatchingProjectResultNotClosed(t *testing.T) {
	mock := &mockVCS{}
	n := newNotification(mock, true, true)

	openIssues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
	}

	// Current run has no result for infra/prod at all
	results := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:   models.TypedProject{Dir: "infra/staging"},
				Drifted:   false,
				Succeeded: true,
			},
		},
	}

	state, err := n.HandleIssues(context.Background(), results, openIssues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.closedIssueNumbers) != 0 {
		t.Errorf("expected no issues closed, got %v", mock.closedIssueNumbers)
	}
	if len(state.DriftIssuesResolved) != 0 {
		t.Errorf("expected no drift issues resolved, got %d", len(state.DriftIssuesResolved))
	}
}

func TestGetProjectIssuesFromGHIssueBodiesSkipsNonDriftiveIssues(t *testing.T) {
	issues := []*vcstypes.VCSIssue{
		{Number: 1, Title: "drift detected: infra/prod", Body: makeIssueBody("infra/prod", "drift")},
		{Number: 2, Title: "Some random issue", Body: "This is a plain issue with no driftive metadata"},
		{Number: 3, Title: "plan error: infra/staging", Body: makeIssueBody("infra/staging", "error")},
		{Number: 4, Title: "Feature request", Body: "Please add support for X"},
	}

	result := getProjectIssuesFromGHIssueBodies(issues)

	if len(result) != 2 {
		t.Fatalf("expected 2 driftive issues, got %d", len(result))
	}
	if result[0].Issue.Number != 1 {
		t.Errorf("expected first result to be issue 1, got %d", result[0].Issue.Number)
	}
	if result[1].Issue.Number != 3 {
		t.Errorf("expected second result to be issue 3, got %d", result[1].Issue.Number)
	}
}

func TestGetProjectFromIssueBody_Valid(t *testing.T) {
	body := makeIssueBody("infra/prod", "drift")
	project, err := getProjectFromIssueBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Project.Dir != "infra/prod" {
		t.Errorf("expected dir infra/prod, got %s", project.Project.Dir)
	}
	if project.Kind != "drift" {
		t.Errorf("expected kind drift, got %s", project.Kind)
	}
}

func TestGetProjectFromIssueBody_NoMetadata(t *testing.T) {
	_, err := getProjectFromIssueBody("just a regular issue body")
	if err == nil {
		t.Error("expected error for body without metadata")
	}
}

func TestGetProjectFromIssueBody_InvalidJSON(t *testing.T) {
	body := "<!--PROJECT_JSON_START-->not-json<!--PROJECT_JSON_END-->"
	_, err := getProjectFromIssueBody(body)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
