package github

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/notification/github/types"
	"fmt"
	"github.com/google/go-github/v67/github"
	"github.com/rs/zerolog/log"
	"strings"
)

type CreateOrUpdateResult struct {
	Created     bool
	RateLimited bool
	Issue       *github.Issue
}

// CreateOrUpdateIssue creates a new issue if it doesn't exist, or updates the existing issue if it does. It returns true if a new issue was created.
func (g *GithubIssueNotification) CreateOrUpdateIssue(
	ctx context.Context,
	driftiveIssue types.GithubIssue,
	openIssues []*github.Issue,
	updateOnly bool) CreateOrUpdateResult {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return CreateOrUpdateResult{
			Created:     false,
			RateLimited: false,
			Issue:       nil,
		}
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == driftiveIssue.Title {
			if issue.GetBody() == driftiveIssue.Body {
				log.Info().Msgf("Issue [%s] already exists for project %s (repo: %s/%s)",
					driftiveIssue.Kind,
					driftiveIssue.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])
				return CreateOrUpdateResult{
					Created:     false,
					RateLimited: false,
					Issue:       nil,
				}
			} else {
				_, _, err := g.ghClient.Issues.Edit(
					ctx,
					ownerRepo[0],
					ownerRepo[1],
					issue.GetNumber(),
					&github.IssueRequest{
						Body: &driftiveIssue.Body,
					})

				if err != nil {
					log.Error().Msgf("Failed to update issue. %v", err)
					return CreateOrUpdateResult{
						Created:     false,
						RateLimited: false,
						Issue:       nil,
					}
				}

				log.Info().Msgf("Updated issue [%s] for project %s (repo: %s/%s)",
					driftiveIssue.Kind,
					driftiveIssue.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])

				return CreateOrUpdateResult{
					Created:     false,
					RateLimited: false,
					Issue:       nil,
				}
			}
		}
	}

	if updateOnly {
		log.Warn().Msgf("Max number of open issues reached. Skipping issue [%s] creation for project %s (repo: %s/%s)",
			driftiveIssue.Kind,
			driftiveIssue.Project.Dir,
			ownerRepo[0],
			ownerRepo[1])
		return CreateOrUpdateResult{
			Created:     false,
			RateLimited: true,
			Issue:       nil,
		}
	}

	ghLabels := driftiveIssue.Labels
	if len(ghLabels) == 0 {
		ghLabels = make([]string, 0)
	}

	issue := &github.IssueRequest{
		Title:  &driftiveIssue.Title,
		Body:   &driftiveIssue.Body,
		Labels: &ghLabels,
	}

	log.Info().Msgf("Creating issue [%s] for project %s (repo: %s/%s)",
		driftiveIssue.Kind,
		driftiveIssue.Project.Dir,
		ownerRepo[0],
		ownerRepo[1])

	createdIssue, _, err := g.ghClient.Issues.Create(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
		issue)

	if err != nil {
		log.Error().Msgf("Failed to create issue. %v", err)
	}
	return CreateOrUpdateResult{
		Created:     true,
		RateLimited: false,
		Issue:       createdIssue,
	}
}

func (g *GithubIssueNotification) GetAllOpenRepoIssues(ctx context.Context) ([]*github.Issue, error) {
	var openIssues []*github.Issue
	opt := &github.IssueListByRepoOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Split owner/repository_name
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("invalid repository name")
	}

	for {
		issues, resp, err := g.ghClient.Issues.ListByRepo(
			ctx,
			ownerRepo[0],
			ownerRepo[1],
			opt)

		if err != nil {
			return nil, err
		}

		openIssues = append(openIssues, issues...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return openIssues, nil
}

func (g *GithubIssueNotification) CloseIssueIfExists(openIssues []*github.Issue, project drift.DriftProjectResult, issueTitle string) bool {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return false
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == issueTitle {
			ctx := context.Background()

			log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
				project.Project.Dir,
				ownerRepo[0],
				ownerRepo[1])

			if _, _, err := g.ghClient.Issues.CreateComment(ctx, ownerRepo[0], ownerRepo[1], issue.GetNumber(), &github.IssueComment{
				Body: github.String("Issue has been resolved."),
			}); err != nil {
				log.Error().Msgf("Failed to comment on issue. %v", err)
			}

			_, _, err := g.ghClient.Issues.Edit(
				ctx,
				ownerRepo[0],
				ownerRepo[1],
				issue.GetNumber(),
				&github.IssueRequest{
					State: github.String("closed"),
				})

			if err != nil {
				log.Error().Msgf("Failed to close issue. %v", err)
				return false
			}
			log.Info().Msgf("Closed issue for project %s (repo: %s/%s)", project.Project.Dir, ownerRepo[0], ownerRepo[1])
			return true
		}
	}
	return false
}

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

	if _, _, err := g.ghClient.Issues.CreateComment(ctx, ownerRepo[0], ownerRepo[1], projectIssue.Issue.GetNumber(), &github.IssueComment{
		Body: github.String("Issue has been resolved."),
	}); err != nil {
		log.Error().Msgf("Failed to comment on issue. %v", err)
	}

	_, _, err := g.ghClient.Issues.Edit(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
		projectIssue.Issue.GetNumber(),
		&github.IssueRequest{
			State: github.String("closed"),
		})

	if err != nil {
		log.Error().Msgf("Failed to close issue. %v", err)
		return false
	}
	log.Info().Msgf("Closed issue [%s] for project %s (repo: %s/%s)", projectIssue.Kind, projectIssue.Project.Dir, ownerRepo[0], ownerRepo[1])
	return true
}
