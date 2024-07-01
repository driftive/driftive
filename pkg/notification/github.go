package notification

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/drift"
	"fmt"
	"github.com/google/go-github/v62/github"
	"github.com/rs/zerolog/log"
)

const (
	issueTitleFormat = "drift detected: %s"
	issueBodyFormat  = "Drift detected in project: %s"
)

type GithubIssueNotification struct {
	config *config.Config
}

func NewGithubIssueNotification(config *config.Config) *GithubIssueNotification {
	return &GithubIssueNotification{config: config}
}

func (g *GithubIssueNotification) CreateOrUpdateIssue(client *github.Client, openIssues []*github.Issue, project drift.DriftProjectResult) {
	ctx := context.Background()

	issueTitle := fmt.Sprintf(issueTitleFormat, project.Project)
	issueBody := fmt.Sprintf(issueBodyFormat, project.Project)

	for _, issue := range openIssues {
		if issue.GetTitle() == issueTitle && issue.GetBody() == issueBody {
			log.Info().Msgf("Issue already exists for project %s (repo: %s/%s)",
				project.Project,
				g.config.GithubContext.RepositoryOwner,
				g.config.GithubContext.Repository)
			return
		}
	}

	issue := &github.IssueRequest{
		Title: &issueTitle,
		Body:  &issueBody,
	}

	log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
		project.Project,
		g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.Repository)

	_, _, err := client.Issues.Create(
		ctx,
		g.config.GithubContext.RepositoryOwner,
		g.config.GithubContext.Repository,
		issue)

	if err != nil {
		log.Error().Msgf("Failed to create issue. %v", err)
	}
}

func (g *GithubIssueNotification) GetAllOpenRepoIssues(client *github.Client) ([]*github.Issue, error) {
	var openIssues []*github.Issue
	ctx := context.Background()
	opt := &github.IssueListByRepoOptions{
		State: "open",
	}
	opt.PerPage = 100

	var issues []*github.Issue
	var err error

	for {
		issues, _, err = client.Issues.ListByRepo(
			ctx,
			g.config.GithubContext.RepositoryOwner,
			g.config.GithubContext.Repository,
			opt)

		if err != nil {
			return nil, err
		}

		openIssues = append(openIssues, issues...)

		if len(issues) == 0 {
			break
		}
		opt.Page++
	}

	return openIssues, nil
}

func (g *GithubIssueNotification) Send(driftResult drift.DriftDetectionResult) {
	if g.config.GithubContext.Repository == "" || g.config.GithubContext.RepositoryOwner == "" {
		log.Warn().Msg("Github repository or owner not provided. Skipping github notification")
		return
	}

	ghClient := github.NewClient(nil).WithAuthToken(g.config.GithubToken)
	openIssues, err := g.GetAllOpenRepoIssues(ghClient)
	if err != nil {
		log.Error().Msgf("Failed to get open issues. %v", err)
		return
	}

	for _, project := range driftResult.DriftedProjects {
		if project.Drifted {
			g.CreateOrUpdateIssue(ghClient, openIssues, project)
		} else if project.Succeeded {
			g.DeleteIssueIfExist(ghClient, openIssues, project)
		}
	}
}

func (g *GithubIssueNotification) DeleteIssueIfExist(client *github.Client, issues []*github.Issue, project drift.DriftProjectResult) {
	for _, issue := range issues {
		if issue.GetTitle() == fmt.Sprintf(issueTitleFormat, project.Project) {
			ctx := context.Background()

			log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
				project.Project,
				g.config.GithubContext.RepositoryOwner,
				g.config.GithubContext.Repository)

			_, _, err := client.Issues.Edit(
				ctx,
				g.config.GithubContext.RepositoryOwner,
				g.config.GithubContext.Repository,
				issue.GetNumber(),
				&github.IssueRequest{
					State: github.String("closed"),
				})

			if err != nil {
				log.Error().Msgf("Failed to close issue. %v", err)
			}
		}
	}
}
