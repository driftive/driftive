package github

import (
	"context"
	"driftive/pkg/notification/github/types"
	"github.com/google/go-github/v69/github"
	"github.com/rs/zerolog/log"
	"strings"
)

func (g *GithubIssueNotification) CloseIssue(ctx context.Context, projectIssue types.ProjectIssue) bool {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return false
	}

	log.Info().Msgf("Closing issue [%s] for project %s (repo: %s/%s)",
		projectIssue.Kind,
		projectIssue.Project.Dir,
		ownerRepo[0],
		ownerRepo[1])

	if _, _, err := g.ghClient.Issues.CreateComment(ctx, ownerRepo[0], ownerRepo[1], projectIssue.Issue.Number, &github.IssueComment{
		Body: github.Ptr("Issue has been resolved."),
	}); err != nil {
		log.Error().Msgf("Failed to comment on issue. %v", err)
	}

	_, _, err := g.ghClient.Issues.Edit(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
		projectIssue.Issue.Number,
		&github.IssueRequest{
			State: github.Ptr("closed"),
		})

	if err != nil {
		log.Error().Msgf("Failed to close issue. %v", err)
		return false
	}
	log.Info().Msgf("Closed issue [%s] for project %s (repo: %s/%s)", projectIssue.Kind, projectIssue.Project.Dir, ownerRepo[0], ownerRepo[1])
	return true
}
