package notification

import (
	"driftive/pkg/config"
	"driftive/pkg/gh"
	"driftive/pkg/models"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	"testing"
)

func issue(dir string, number int, kind string) types.ProjectIssue {
	return types.ProjectIssue{
		Project: models.Project{Dir: dir},
		Issue:   vcstypes.VCSIssue{Number: number},
		Kind:    kind,
	}
}

func TestIssuesStateFromGithubPopulatesAllFourCounters(t *testing.T) {
	state := &types.GithubState{
		DriftIssuesOpen:     []types.ProjectIssue{issue("a", 1, types.DriftIssueKind), issue("b", 2, types.DriftIssueKind)},
		DriftIssuesResolved: []types.ProjectIssue{issue("c", 3, types.DriftIssueKind)},
		ErrorIssuesOpen:     []types.ProjectIssue{issue("d", 4, types.ErrorIssueKind)},
		ErrorIssuesResolved: []types.ProjectIssue{
			issue("e", 5, types.ErrorIssueKind),
			issue("f", 6, types.ErrorIssueKind),
		},
	}

	got := issuesStateFromGithub(state)

	if !got.StateUpdated {
		t.Error("StateUpdated = false, want true")
	}
	if got.NumOpenIssues != 2 {
		t.Errorf("NumOpenIssues = %d, want 2", got.NumOpenIssues)
	}
	if got.NumResolvedIssues != 1 {
		t.Errorf("NumResolvedIssues = %d, want 1", got.NumResolvedIssues)
	}
	if got.NumOpenErrorIssues != 1 {
		t.Errorf("NumOpenErrorIssues = %d, want 1", got.NumOpenErrorIssues)
	}
	if got.NumResolvedErrorIssues != 2 {
		t.Errorf("NumResolvedErrorIssues = %d, want 2", got.NumResolvedErrorIssues)
	}
}

// TestIssuesStateFromGithubNilStateIsNotUpdated keeps a failed or disabled GitHub run from
// producing a green "all resolved" Slack message.
func TestIssuesStateFromGithubNilStateIsNotUpdated(t *testing.T) {
	got := issuesStateFromGithub(nil)

	if got.StateUpdated {
		t.Error("StateUpdated = true, want false")
	}
	if got.NumResolvedIssues != -1 || got.NumResolvedErrorIssues != -1 {
		t.Errorf("resolved counters = %d/%d, want -1/-1", got.NumResolvedIssues, got.NumResolvedErrorIssues)
	}
	if got.NumOpenIssues != -1 || got.NumOpenErrorIssues != -1 {
		t.Errorf("open counters = %d/%d, want -1/-1", got.NumOpenIssues, got.NumOpenErrorIssues)
	}
}

func TestRepoSlug(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.DriftiveConfig
		want string
	}{
		{
			name: "with github context",
			cfg:  &config.DriftiveConfig{GithubContext: &gh.GithubActionContext{Repository: "acme/infra"}},
			want: "acme/infra",
		},
		{
			name: "without github context",
			cfg:  &config.DriftiveConfig{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repoSlug(tt.cfg); got != tt.want {
				t.Errorf("repoSlug() = %q, want %q", got, tt.want)
			}
		})
	}
}
