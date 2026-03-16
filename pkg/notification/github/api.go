package github

import (
	"context"
	"driftive/pkg/notification/github/types"

	"github.com/rs/zerolog/log"
)

func (g *GithubIssueNotification) CloseIssueWithComment(ctx context.Context,
	projectIssue types.ProjectIssue) bool {
	log.Info().Msgf("Closing issue [%s] for project %s (repo: %s/%s)",
		projectIssue.Kind,
		projectIssue.Project.Dir,
		g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.GetRepositoryName())

	err := g.scm.CreateIssueComment(ctx, projectIssue.Issue.Number)
	if err != nil {
		log.Error().Msgf("Failed to create issue comment. %v", err)
		return false
	}

	err = g.scm.CloseIssue(ctx, projectIssue.Issue.Number)
	if err != nil {
		log.Error().Msgf("Failed to close issue. %v", err)
		return false
	}

	log.Info().Msgf("Closed issue [%s] for project %s (repo: %s/%s)",
		projectIssue.Kind, projectIssue.Project.Dir, g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.GetRepositoryName())
	return true
}

func (g *GithubPullRequestNotification) ClosePullRequestWithComment(ctx context.Context,
	projectPullRequest types.ProjectPullRequest) bool {
	log.Info().Msgf("Closing pull request [%s] for project %s (repo: %s/%s)",
		projectPullRequest.Kind,
		projectPullRequest.Project.Dir,
		g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.GetRepositoryName())

	err := g.scm.CreatePullRequestComment(ctx, projectPullRequest.Pr.Number, "Pull request has been resolved.")
	if err != nil {
		log.Error().Msgf("Failed to create pull request comment. %v", err)
		return false
	}

	err = g.scm.ClosePullRequest(ctx, projectPullRequest.Pr.Number)
	if err != nil {
		log.Error().Msgf("Failed to close pull request. %v", err)
		return false
	}

	log.Info().Msgf("Closed pull request [%s] for project %s (repo: %s/%s)",
		projectPullRequest.Kind, projectPullRequest.Project.Dir, g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.GetRepositoryName())
	return true
}
