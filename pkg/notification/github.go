package notification

import (
	"bytes"
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/models/backend"
	"driftive/pkg/utils"
	"errors"
	"fmt"
	"github.com/google/go-github/v64/github"
	"github.com/rs/zerolog/log"
	"strings"
	"text/template"
)

const (
	issueTitleFormat       = "drift detected: %s"
	errorIssueTitleFormat  = "plan error: %s"
	maxIssueBodySize       = 64000 // Lower than 65535 to account for other metadata
	issueBodyTemplate      = "State drift in project: {{ .ProjectDir }}\n\n<details>\n<summary>Output</summary>\n\n```hcl\n\n{{ .Output }}\n\n```\n\n</details>"
	errorIssueBodyTemplate = "Error in project: {{ .ProjectDir }}\n\n<details>\n<summary>Output</summary>\n\n```hcl\n\n{{ .Output }}\n\n```\n\n</details>"
	ErrRepoNotProvided     = "repository or owner not provided"
	ErrGHTokenNotProvided  = "github token not provided"
	titleKeyword           = "drift detected"
	errorTitleKeyword      = "plan error"
)

type GithubIssueNotification struct {
	config     *config.DriftiveConfig
	repoConfig *repo.DriftiveRepoConfig
	ghClient   *github.Client
}

type GithubIssue struct {
	Title   string
	Body    string
	Labels  []string
	Project models.Project
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

func parseGithubBodyTemplate(project drift.DriftProjectResult, bodyTemplate string) (*string, error) {
	templateArgs := struct {
		ProjectDir string
		Output     string
	}{
		ProjectDir: project.Project.Dir,
		Output:     project.PlanOutput[0:utils.Min(len(project.PlanOutput), maxIssueBodySize)],
	}

	tmpl, err := template.New("gh-issue").Parse(strings.Trim(bodyTemplate, " \n"))
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

func (g *GithubIssueNotification) Send(driftResult drift.DriftDetectionResult) (*backend.DriftIssuesState, error) {
	allOpenIssues, err := g.GetAllOpenRepoIssues()
	if err != nil {
		log.Error().Msgf("Failed to get open issues. %v", err)
		return nil, err
	}

	driftOpenIssues := countIssuesByLabelsOrTitle(allOpenIssues, g.repoConfig.GitHub.Issues.Labels, titleKeyword)
	initialOpenIssues := driftOpenIssues
	if g.repoConfig.GitHub.Issues.CloseResolved {
		for _, project := range driftResult.ProjectResults {
			if !project.Drifted && project.Succeeded {
				closed := g.CloseIssueIfExists(allOpenIssues, project, fmt.Sprintf(issueTitleFormat, project.Project.Dir))
				if closed {
					driftOpenIssues--
				}
			}
		}
	}

	totalResolvedIssues := initialOpenIssues - driftOpenIssues
	// Create issues for drifted projects
	for _, projectResult := range driftResult.ProjectResults {
		if projectResult.Drifted {

			issueBody, err := parseGithubBodyTemplate(projectResult, issueBodyTemplate)
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse github issue description template")
				continue
			}

			issue := GithubIssue{
				Title:   fmt.Sprintf(issueTitleFormat, projectResult.Project.Dir),
				Body:    *issueBody,
				Labels:  g.repoConfig.GitHub.Issues.Labels,
				Project: projectResult.Project,
			}
			created := g.CreateOrUpdateIssue(
				issue,
				allOpenIssues,
				driftOpenIssues >= g.repoConfig.GitHub.Issues.MaxOpenIssues,
			)
			if created {
				driftOpenIssues++
			}
		}
	}

	errorOpenIssues := countIssuesByLabelsOrTitle(allOpenIssues, g.repoConfig.GitHub.Issues.Errors.Labels, errorTitleKeyword)
	initialErrorOpenIssues := errorOpenIssues
	if g.repoConfig.GitHub.Issues.Errors.CloseResolved {
		for _, project := range driftResult.ProjectResults {
			if !project.Drifted && !project.Succeeded {
				closed := g.CloseIssueIfExists(allOpenIssues, project, fmt.Sprintf(errorIssueTitleFormat, project.Project.Dir))
				if closed {
					errorOpenIssues--
				}
			}
		}
	}
	totalResolvedErrorIssues := initialErrorOpenIssues - errorOpenIssues

	// Create issues for failed projects
	// TODO validate if there are error labels being used in drift labels during config time!
	if g.repoConfig.GitHub.Issues.Errors.Enabled {
		for _, projectResult := range driftResult.ProjectResults {
			if !projectResult.Succeeded && !projectResult.Drifted {
				issueBody, err := parseGithubBodyTemplate(projectResult, errorIssueBodyTemplate)
				if err != nil {
					log.Error().Err(err).Msg("Failed to parse github issue description template")
					continue
				}

				issue := GithubIssue{
					Title:   fmt.Sprintf(errorIssueTitleFormat, projectResult.Project.Dir),
					Body:    *issueBody,
					Labels:  g.repoConfig.GitHub.Issues.Errors.Labels,
					Project: projectResult.Project,
				}
				created := g.CreateOrUpdateIssue(
					issue,
					allOpenIssues,
					errorOpenIssues >= g.repoConfig.GitHub.Issues.Errors.MaxOpenIssues,
				)
				if created {
					errorOpenIssues++
				}
			}
		}
	}

	return &backend.DriftIssuesState{
		NumOpenIssues:          driftOpenIssues,
		NumResolvedIssues:      totalResolvedIssues,
		NumOpenErrorIssues:     errorOpenIssues,
		NumResolvedErrorIssues: totalResolvedErrorIssues,
		StateUpdated:           true,
	}, nil
}

func containsAnyLabel(issue *github.Issue, labels []string) bool {
	for _, label := range issue.Labels {
		for _, l := range labels {
			if l == label.GetName() {
				return true
			}
		}
	}
	return false
}

// countIssuesByLabelsOrTitle counts the number of issues that have any of the labels or the title contains the keyword
func countIssuesByLabelsOrTitle(issues []*github.Issue, labels []string, titleKeyword string) int {
	count := 0
	for _, issue := range issues {
		if containsAnyLabel(issue, labels) {
			count++
			continue
		}
		if strings.Contains(issue.GetTitle(), titleKeyword) {
			count++
			continue
		}
	}
	return count
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
