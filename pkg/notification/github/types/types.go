package types

import (
	"driftive/pkg/models"
	"driftive/pkg/vcs/vcstypes"
	"time"
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

type ProjectPullRequest struct {
	Project models.Project          `json:"project" yaml:"project"`
	Pr      vcstypes.VCSPullRequest `json:"pr" yaml:"pr"`
	Kind    string                  `json:"kind" yaml:"kind" validate:"oneof=drift error"`
}

type GithubPullRequest struct {
	Title   string
	Body    string
	Labels  []string
	Branch  string
	Base    string
	Project models.TypedProject
	Kind    string
	Time    time.Time
}

type GithubState struct {
	DriftIssuesOpen           []ProjectIssue
	DriftIssuesResolved       []ProjectIssue
	DriftPullRequestsOpen     []ProjectPullRequest
	DriftPullRequestsResolved []ProjectPullRequest

	ErrorIssuesOpen     []ProjectIssue
	ErrorIssuesResolved []ProjectIssue

	RateLimitedDrifts []string
}
