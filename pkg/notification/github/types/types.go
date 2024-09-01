package types

import (
	"driftive/pkg/models"
	"github.com/google/go-github/v64/github"
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
	Project models.Project `json:"project" yaml:"project"`
	Issue   github.Issue   `json:"issue" yaml:"issue"`
	Kind    string         `json:"kind" yaml:"kind" validate:"oneof=drift error"`
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
}
