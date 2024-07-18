package notification

import (
	"bytes"
	"context"
	"driftive/pkg/config"
	"driftive/pkg/drift"
	"driftive/pkg/utils"
	"fmt"
	"github.com/google/go-github/v63/github"
	"github.com/rs/zerolog/log"
	"strings"
	"text/template"
)

const (
	issueTitleFormat = "drift detected: %s"
	maxIssueBodySize = 64000 // Lower than 65535 to account for other metadata

	issueBodyTemplate = "State drift in project: {{ .ProjectDir }}\n\n<details>\n<summary>Output</summary>\n\n```hcl\n\n{{ .Output }}\n\n```\n\n</details>"
)

type GithubIssueNotification struct {
	config *config.DriftiveConfig
}

func NewGithubIssueNotification(config *config.DriftiveConfig) *GithubIssueNotification {
	return &GithubIssueNotification{config: config}
}

func parseGithubBodyTemplate(project drift.DriftProjectResult) (*string, error) {
	templateArgs := struct {
		ProjectDir string
		Output     string
	}{
		ProjectDir: project.Project.Dir,
		Output:     project.PlanOutput[0:utils.Min(len(project.PlanOutput), maxIssueBodySize)],
	}

	tmpl, err := template.New("gh-issue").Parse(strings.Trim(issueBodyTemplate, " \n"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse github issue description template")
		return nil, err
	}
	buff := new(bytes.Buffer)
	err = tmpl.Execute(buff, templateArgs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute github issue description template")
		return nil, err
	}
	resultStr := buff.String()
	return &resultStr, nil
}

// CreateOrUpdateIssue creates a new issue if it doesn't exist, or updates the existing issue if it does. It returns true if a new issue was created.
func (g *GithubIssueNotification) CreateOrUpdateIssue(client *github.Client, openIssues []*github.Issue, project drift.DriftProjectResult, openIssueCount int) bool {
	ctx := context.Background()

	issueTitle := fmt.Sprintf(issueTitleFormat, project.Project.Dir)

	issueBody, err := parseGithubBodyTemplate(project)

	if err != nil {
		log.Error().Err(err).Msg("Failed to parse github issue description template")
		return false
	}

	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return false
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == issueTitle {
			if issue.GetBody() == *issueBody {
				log.Info().Msgf("Issue already exists for project %s (repo: %s/%s)",
					project.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])
				return false
			} else {
				_, _, err := client.Issues.Edit(
					ctx,
					ownerRepo[0],
					ownerRepo[1],
					issue.GetNumber(),
					&github.IssueRequest{
						Body: issueBody,
					})

				if err != nil {
					log.Error().Msgf("Failed to update issue. %v", err)
					return false
				}

				log.Info().Msgf("Updated issue for project %s (repo: %s/%s)",
					project.Project.Dir,
					ownerRepo[0],
					ownerRepo[1])

				return false
			}
		}
	}

	if openIssueCount >= g.config.MaxOpenedIssues {
		log.Warn().Msgf("Max number of open issues reached. Skipping issue creation for project %s (repo: %s/%s)",
			project.Project.Dir,
			ownerRepo[0],
			ownerRepo[1])
		return false
	}

	issue := &github.IssueRequest{
		Title: &issueTitle,
		Body:  issueBody,
	}

	log.Info().Msgf("Creating issue for project %s (repo: %s/%s)",
		project.Project.Dir,
		ownerRepo[0],
		ownerRepo[1])

	_, _, err = client.Issues.Create(
		ctx,
		ownerRepo[0],
		ownerRepo[1],
		issue)

	if err != nil {
		log.Error().Msgf("Failed to create issue. %v", err)
	}
	return true
}

func (g *GithubIssueNotification) GetAllOpenRepoIssues(client *github.Client) ([]*github.Issue, error) {
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
		issues, resp, err := client.Issues.ListByRepo(
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

	driftiveOpenIssues := countDriftiveOpenIssues(openIssues)
	for _, project := range driftResult.DriftedProjects {
		if g.config.CloseResolvedIssues && !project.Drifted && project.Succeeded {
			closed := g.CloseIssueIfExists(ghClient, openIssues, project)
			if closed {
				driftiveOpenIssues--
			}
		}
	}

	for _, project := range driftResult.DriftedProjects {
		if project.Drifted {
			created := g.CreateOrUpdateIssue(ghClient, openIssues, project, driftiveOpenIssues)
			if created {
				driftiveOpenIssues++
			}
		}
	}
}

func countDriftiveOpenIssues(issues []*github.Issue) int {
	count := 0
	for _, issue := range issues {
		if strings.Contains(issue.GetTitle(), "drift detected") {
			count++
		}
	}
	return count
}

func (g *GithubIssueNotification) CloseIssueIfExists(client *github.Client, openIssues []*github.Issue, project drift.DriftProjectResult) bool {
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name")
		return false
	}

	for _, issue := range openIssues {
		if issue.GetTitle() == fmt.Sprintf(issueTitleFormat, project.Project.Dir) {
			ctx := context.Background()

			log.Info().Msgf("Closing issue for project %s (repo: %s/%s)",
				project.Project.Dir,
				ownerRepo[0],
				ownerRepo[1])

			if _, _, err := client.Issues.CreateComment(ctx, ownerRepo[0], ownerRepo[1], issue.GetNumber(), &github.IssueComment{
				Body: github.String("Drift has been resolved."),
			}); err != nil {
				log.Error().Msgf("Failed to comment on issue. %v", err)
			}

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
				return false
			}
			log.Info().Msgf("Closed issue for project %s (repo: %s/%s)", project.Project.Dir, ownerRepo[0], ownerRepo[1])
			return true
		}
	}
	return false
}
