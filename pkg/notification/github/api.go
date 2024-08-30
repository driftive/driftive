package github

import (
	"context"
	"fmt"
	"github.com/google/go-github/v64/github"
	"github.com/rs/zerolog/log"
	"strings"
)

// CreateOrUpdateIssue creates a new issue if it doesn't exist, or updates the existing issue if it does. It returns true if a new issue was created.
func (g *GithubIssueNotification) CreateOrUpdateIssue(
	driftiveIssue GithubIssue,
	openIssues []*github.Issue,
	updateOnly bool) bool {
	ctx := context.Background()

	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return false
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == driftiveIssue.Title {
			if issue.GetBody() == driftiveIssue.Body {
				log.Info().Msgf("Issue already exists for project %s (repo: %s/%s)",
					driftiveIssue.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])
				return false
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
					return false
				}

				log.Info().Msgf("Updated issue for project %s (repo: %s/%s)",
					driftiveIssue.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])

				return false
			}
		}
	}

	if updateOnly {
		log.Warn().Msgf("Max number of open issues reached. Skipping issue creation for project %s (repo: %s/%s)",
			driftiveIssue.Project.Dir,
			ownerRepo[0],
			ownerRepo[1])
		return false
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

	log.Info().Msgf("Creating issue for project %s (repo: %s/%s)",
		driftiveIssue.Project.Dir,
		ownerRepo[0],
		ownerRepo[1])

	_, _, err := g.ghClient.Issues.Create(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
		issue)

	if err != nil {
		log.Error().Msgf("Failed to create issue. %v", err)
	}
	return true
}

func (g *GithubIssueNotification) GetAllOpenRepoIssues() ([]*github.Issue, error) {
	var openIssues []*github.Issue
	ctx := context.Background()
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
