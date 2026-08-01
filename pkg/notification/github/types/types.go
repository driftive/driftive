package types

import (
	"driftive/pkg/models"
	"driftive/pkg/vcs/vcstypes"
)

const (
	DriftIssueKind = "drift"
	ErrorIssueKind = "error"
)

// GHProject represents a project with its kind. This type is stored in GH issue body
type GHProject struct {
	Project models.Project `json:"project" yaml:"project"`
	Kind    string         `json:"kind" yaml:"kind" validate:"oneof=drift error"`
}

type ProjectIssue struct {
	Project models.Project    `json:"project" yaml:"project"`
	Issue   vcstypes.VCSIssue `json:"issue" yaml:"issue"`
	Kind    string            `json:"kind" yaml:"kind" validate:"oneof=drift error"`
}

type GithubIssue struct {
	Title   string
	Body    string
	Labels  []string
	Project models.TypedProject
	Kind    string
}

type GithubState struct {
	DriftIssuesOpen     []ProjectIssue
	DriftIssuesResolved []ProjectIssue

	ErrorIssuesOpen     []ProjectIssue
	ErrorIssuesResolved []ProjectIssue

	RateLimitedDrifts []string
	RateLimitedErrors []string
}

// IssueNumbersByDir indexes the issues still open after this run by project dir, for the given
// kind. Returns nil when there is no state, so callers can pass the result straight through.
func (s *GithubState) IssueNumbersByDir(kind string) map[string]int {
	if s == nil {
		return nil
	}

	issues := s.DriftIssuesOpen
	if kind == ErrorIssueKind {
		issues = s.ErrorIssuesOpen
	}

	numbers := make(map[string]int, len(issues))
	for _, issue := range issues {
		numbers[issue.Project.Dir] = issue.Issue.Number
	}
	return numbers
}
