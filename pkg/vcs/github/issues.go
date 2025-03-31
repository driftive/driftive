package github

import (
	"context"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	"fmt"
	"github.com/google/go-github/v70/github"
	"github.com/rs/zerolog/log"
	"strings"
)

func (g *GHOps) toSCMIssues(issues []*github.Issue) []*vcstypes.VCSIssue {
	if len(issues) == 0 {
		return make([]*vcstypes.VCSIssue, 0)
	}
	scmIssues := make([]*vcstypes.VCSIssue, 0)
	for _, issue := range issues {
		scmIssues = append(scmIssues, g.toSCMIssue(issue))
	}
	return scmIssues
}

func (g *GHOps) toSCMIssue(issue *github.Issue) *vcstypes.VCSIssue {
	if issue == nil {
		return nil
	}
	return &vcstypes.VCSIssue{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   issue.GetBody(),
	}
}

func (g *GHOps) GetAllOpenRepoIssues(ctx context.Context) ([]*vcstypes.VCSIssue, error) {
	log.Info().Msg("Fetching all open issues from the repository...")
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

	issues := g.toSCMIssues(openIssues)
	log.Info().Msgf("Fetched %d open issues from the repository", len(issues))
	return issues, nil
}

// CreateOrUpdateIssue creates a new issue if it doesn't exist, or updates the existing issue if it does. It returns true if a new issue was created.
func (g *GHOps) CreateOrUpdateIssue(
	ctx context.Context,
	driftiveIssue types.GithubIssue,
	openIssues []*vcstypes.VCSIssue,
	updateOnly bool) vcstypes.CreateOrUpdateResult {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return vcstypes.CreateOrUpdateResult{
			Created:     false,
			RateLimited: false,
			Issue:       nil,
		}
	}

	for _, issue := range openIssues {
		if issue.Title == driftiveIssue.Title {
			if issue.Body == driftiveIssue.Body {
				log.Info().Msgf("Issue [%s] already exists for project %s (repo: %s/%s)",
					driftiveIssue.Kind,
					driftiveIssue.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])
				return vcstypes.CreateOrUpdateResult{
					Created:     false,
					RateLimited: false,
					Issue:       nil,
				}
			} else {
				_, _, err := g.ghClient.Issues.Edit(
					ctx,
					ownerRepo[0],
					ownerRepo[1],
					issue.Number,
					&github.IssueRequest{
						Body: &driftiveIssue.Body,
					})

				if err != nil {
					log.Error().Msgf("Failed to update issue. %v", err)
					return vcstypes.CreateOrUpdateResult{
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

				return vcstypes.CreateOrUpdateResult{
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
		return vcstypes.CreateOrUpdateResult{
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
	return vcstypes.CreateOrUpdateResult{
		Created:     true,
		RateLimited: false,
		Issue:       g.toSCMIssue(createdIssue),
	}
}

func (g *GHOps) CreateIssueComment(
	ctx context.Context,
	issueNumber int,
) error {
	owner := g.config.GithubContext.RepositoryOwner
	repo := g.config.GithubContext.GetRepositoryName()
	_, resp, err := g.ghClient.Issues.CreateComment(ctx, owner, repo, issueNumber, &github.IssueComment{
		Body: github.Ptr("Issue has been resolved."),
	})
	if err != nil {
		log.Error().Msgf("Failed to comment on issue. %v", err)
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (g *GHOps) CloseIssue(ctx context.Context, issueNumber int) error {
	owner := g.config.GithubContext.RepositoryOwner
	repo := g.config.GithubContext.GetRepositoryName()
	_, issueEditResp, err := g.ghClient.Issues.Edit(
		ctx,
		owner,
		repo,
		issueNumber,
		&github.IssueRequest{
			State: github.Ptr("closed"),
		})
	if err != nil {
		log.Error().Msgf("Failed to close issue. %v", err)
		return err
	}
	defer issueEditResp.Body.Close()
	return nil
}
