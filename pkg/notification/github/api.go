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
