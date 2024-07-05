package notification

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/drift"
	"fmt"
	"github.com/google/go-github/v62/github"
	"github.com/rs/zerolog/log"
	"strings"
)

const (
	issueTitleFormat = "drift detected: %s"
	issueBodyFormat  = "Drift detected in project: %s"
)

type GithubIssueNotification struct {
	config *config.DriftiveConfig
}

func NewGithubIssueNotification(config *config.DriftiveConfig) *GithubIssueNotification {
	return &GithubIssueNotification{config: config}
}

func (g *GithubIssueNotification) CreateOrUpdateIssue(client *github.Client, openIssues []*github.Issue, project drift.DriftProjectResult) {
	ctx := context.Background()

	issueTitle := fmt.Sprintf(issueTitleFormat, project.Project)
	issueBody := fmt.Sprintf(issueBodyFormat, project.Project)

	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == issueTitle && issue.GetBody() == issueBody {
			log.Info().Msgf("Issue already exists for project %s (repo: %s/%s)",
				project.Project,
				ownerRepo[0],
				ownerRepo[1])
			return
		}
	}

	issue := &github.IssueRequest{
		Title: &issueTitle,
		Body:  &issueBody,
	}

	log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
		project.Project,
		ownerRepo[0],
		ownerRepo[1])

	_, _, err := client.Issues.Create(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
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

	// Split owner/repository_name
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("invalid repository name")
	}

	for {
		issues, _, err = client.Issues.ListByRepo(
			ctx,
			ownerRepo[0],
			ownerRepo[1],
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
		} else if !project.Drifted && project.Succeeded {
			g.DeleteIssueIfExists(ghClient, openIssues, project)
		}
	}
}

func (g *GithubIssueNotification) DeleteIssueIfExists(client *github.Client, issues []*github.Issue, project drift.DriftProjectResult) {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return
	}

	for _, issue := range issues {
		if issue.GetTitle() == fmt.Sprintf(issueTitleFormat, project.Project) {
			ctx := context.Background()

			log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
				project.Project,
				ownerRepo[0],
				ownerRepo[1])

			_, _, err := client.Issues.Edit(
				ctx,
				ownerRepo[0],
				ownerRepo[1],
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
