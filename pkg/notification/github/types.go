package github

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/models"
	"errors"
	"github.com/google/go-github/v64/github"
	"github.com/rs/zerolog/log"
)

type IssueKind int

const (
	DriftIssueKind IssueKind = iota
	ErrorIssueKind
)

type GithubIssueNotification struct {
	config     *config.DriftiveConfig
	repoConfig *repo.DriftiveRepoConfig
	ghClient   *github.Client
}

func (g *GithubIssueNotification) closeIssues(ctx context.Context, issues []ProjectIssue) []ProjectIssue {
	if !g.repoConfig.GitHub.Issues.CloseResolved && len(issues) > 0 {
		log.Warn().Msg("Note: There are GH drift issues but driftive is not configured to close them.")
		return []ProjectIssue{}
	}

	var closedIssues []ProjectIssue
	for _, projIssue := range issues {
		closed := g.CloseIssue(ctx, projIssue)
		if closed {
			closedIssues = append(closedIssues, projIssue)
		}
	}

	return closedIssues
}

// GHProject represents a project with its kind. This type is stored in GH issue body
type GHProject struct {
	Project models.Project `json:"project" yaml:"project"`
	Kind    IssueKind      `json:"kind" yaml:"kind"`
}

type ProjectIssue struct {
	Project models.Project
	Issue   github.Issue
	Kind    IssueKind
}

type GithubIssue struct {
	Title   string
	Body    string
	Labels  []string
	Project models.TypedProject
	Kind    IssueKind
}

type GithubState struct {
	DriftIssuesOpen     []models.Project
	DriftIssuesResolved []models.Project

	ErrorIssuesOpen     []models.Project
	ErrorIssuesResolved []models.Project
}

func NewGithubIssueNotification(config *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) (*GithubIssueNotification, error) {
	if config.GithubContext.Repository == "" || config.GithubContext.RepositoryOwner == "" {
		log.Warn().Msg("Github repository or owner not provided. Skipping github notification")
		return nil, errors.New(ErrRepoNotProvided)
	}

	if config.GithubToken == "" {
		log.Warn().Msg("Github token not provided. Skipping github notification")
		return nil, errors.New(ErrGHTokenNotProvided)
	}

	ghClient := github.NewClient(nil).WithAuthToken(config.GithubToken)
	return &GithubIssueNotification{config: config, repoConfig: repoConfig, ghClient: ghClient}, nil
}
