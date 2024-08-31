package github

import (
	"bytes"
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/utils"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-github/v64/github"
	"github.com/rs/zerolog/log"
	"strings"
	"text/template"
)

const (
	issueTitleFormat                 = "drift detected: %s"
	errorIssueTitleFormat            = "plan error: %s"
	maxIssueBodySize                 = 64000 // Lower than 65535 to account for other metadata
	ErrRepoNotProvided               = "repository or owner not provided"
	ErrGHTokenNotProvided            = "github token not provided"
	titleKeyword                     = "drift detected"
	errorTitleKeyword                = "plan error"
	issueBodyProjectNameStartKeyword = "<!--PROJECT_JSON_START-->"
	issueBodyProjectNameEndKeyword   = "<!--PROJECT_JSON_END-->"
)

//go:embed template/gh-issue-description.md
var issueBodyTemplate string

//go:embed template/gh-error-issue-description.md
var errorIssueBodyTemplate string

func parseGithubBodyTemplate(project drift.DriftProjectResult, bodyTemplate string) (*string, error) {

	projectKind := DriftIssueKind
	if !project.Drifted && !project.Succeeded {
		projectKind = ErrorIssueKind
	}

	ghProject := GHProject{
		Project: models.Project{
			Dir: project.Project.Dir,
		},
		Kind: projectKind,
	}

	projectJson, err := json.Marshal(ghProject)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal project to json")
		return nil, err
	}

	templateArgs := struct {
		ProjectDir  string
		Output      string
		ProjectJSON string
	}{
		ProjectDir:  project.Project.Dir,
		Output:      project.PlanOutput[0:utils.Min(len(project.PlanOutput), maxIssueBodySize)],
		ProjectJSON: string(projectJson),
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

func (g *GithubIssueNotification) closeResolvedDriftIssues(allOpenIssues []*github.Issue, result drift.DriftDetectionResult) int {
	if !g.repoConfig.GitHub.Issues.CloseResolved {
		return 0
	}
	closedIssues := 0
	for _, project := range result.ProjectResults {
		if !project.Drifted && project.Succeeded {
			closed := g.CloseIssueIfExists(allOpenIssues, project, fmt.Sprintf(issueTitleFormat, project.Project.Dir))
			if closed {
				closedIssues++
			}
		}
	}
	return closedIssues
}

func (g *GithubIssueNotification) closeResolvedErrorIssues(allOpenIssues []*github.Issue, result drift.DriftDetectionResult) int {
	if !g.repoConfig.GitHub.Issues.Errors.CloseResolved {
		return 0
	}
	closedIssues := 0
	for _, project := range result.ProjectResults {
		if project.Succeeded && g.repoConfig.GitHub.Issues.Errors.CloseResolved {
			closed := g.CloseIssueIfExists(allOpenIssues, project, fmt.Sprintf(errorIssueTitleFormat, project.Project.Dir))
			if closed {
				closedIssues++
			}
		}
	}
	return closedIssues
}

func (g *GithubIssueNotification) Send(ctx context.Context, driftResult drift.DriftDetectionResult) (*GithubState, error) {
	allOpenIssues, err := g.GetAllOpenRepoIssues()
	if err != nil {
		log.Error().Msgf("Failed to get open issues. %v", err)
		return nil, err
	}

	allDriftiveOpenIssues := getProjectIssuesFromGHIssueBodies(allOpenIssues)
	numOpenDriftIssues := 0
	for _, issue := range allDriftiveOpenIssues {
		if issue.Kind == DriftIssueKind {
			numOpenDriftIssues++
		}
	}
	numOpenErrorIssues := 0
	for _, issue := range allDriftiveOpenIssues {
		if issue.Kind == ErrorIssueKind {
			numOpenErrorIssues++
		}
	}

	var closeableDriftIssues []ProjectIssue
	for _, project := range allDriftiveOpenIssues {
		if project.Kind == DriftIssueKind {
			for _, projectResult := range driftResult.ProjectResults {
				if !projectResult.Drifted && project.Project.Dir == projectResult.Project.Dir {
					closeableDriftIssues = append(closeableDriftIssues, project)
				}
			}
		}
	}

	var closeableErrorIssues []ProjectIssue
	for _, project := range allDriftiveOpenIssues {
		if project.Kind == ErrorIssueKind {
			for _, projectResult := range driftResult.ProjectResults {
				if projectResult.Succeeded && project.Project.Dir == projectResult.Project.Dir {
					closeableErrorIssues = append(closeableErrorIssues, project)
				}
			}
		}
	}

	closedDriftIssues := g.closeIssues(ctx, closeableDriftIssues)
	log.Info().Msgf("Closed %d state-drifted issues", len(closedDriftIssues))
	numOpenDriftIssues = numOpenDriftIssues - len(closedDriftIssues)
	var newlyCreatedIssues []ProjectIssue

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
				Kind:    DriftIssueKind,
			}
			created, createdIssue := g.CreateOrUpdateIssue(
				issue,
				allOpenIssues,
				numOpenDriftIssues >= g.repoConfig.GitHub.Issues.MaxOpenIssues,
			)
			if created {
				numOpenDriftIssues++
				newlyCreatedIssues = append(newlyCreatedIssues, ProjectIssue{
					Issue: *createdIssue,
					Project: models.Project{
						Dir: projectResult.Project.Dir,
					},
					Kind: DriftIssueKind,
				})
			}
		}
	}

	closedErrorIssues := g.closeIssues(ctx, closeableErrorIssues)
	log.Info().Msgf("Closed %d errored issues", len(closedErrorIssues))
	numOpenErrorIssues = numOpenErrorIssues - len(closedErrorIssues)

	// Create issues for failed projects
	if g.repoConfig.GitHub.Issues.Errors.Enabled {
		for _, projectResult := range driftResult.ProjectResults {
			if !projectResult.Succeeded {
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
					Kind:    ErrorIssueKind,
				}
				created, createdIssue := g.CreateOrUpdateIssue(
					issue,
					allOpenIssues,
					numOpenErrorIssues >= g.repoConfig.GitHub.Issues.Errors.MaxOpenIssues,
				)
				if created {
					numOpenErrorIssues++
					newlyCreatedIssues = append(newlyCreatedIssues, ProjectIssue{
						Issue: *createdIssue,
						Project: models.Project{
							Dir: projectResult.Project.Dir,
						},
						Kind: ErrorIssueKind,
					})
				}
			}
		}
	}

	currentOpenIssues := append(allDriftiveOpenIssues, newlyCreatedIssues...)
	currentDriftedIssues := filterIssues(filterIssuesByKind(currentOpenIssues, DriftIssueKind), closedDriftIssues)
	currentErroredIssues := filterIssues(filterIssuesByKind(currentOpenIssues, ErrorIssueKind), closedErrorIssues)

	return &GithubState{
		DriftIssuesOpen:     projectIssueListToProjectList(currentDriftedIssues),
		DriftIssuesResolved: projectIssueListToProjectList(closedDriftIssues),
		ErrorIssuesOpen:     projectIssueListToProjectList(currentErroredIssues),
		ErrorIssuesResolved: projectIssueListToProjectList(closedErrorIssues),
	}, nil
}

func filterIssuesByKind(allIssues []ProjectIssue, kind IssueKind) []ProjectIssue {
	var issues []ProjectIssue
	for _, issue := range allIssues {
		if issue.Kind == kind {
			issues = append(issues, issue)
		}
	}
	return issues
}

func projectIssueToProject(projectIssue ProjectIssue) models.Project {
	return models.Project{
		Dir: projectIssue.Project.Dir,
	}
}

func projectIssueListToProjectList(projectIssues []ProjectIssue) []models.Project {
	var projects []models.Project
	for _, projectIssue := range projectIssues {
		projects = append(projects, projectIssueToProject(projectIssue))
	}
	return projects
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

// getProjectIssuesFromGHIssueBodies lists the issues that have any of the labels or the title contains the keyword
func getProjectIssuesFromGHIssueBodies(ghIssues []*github.Issue) []ProjectIssue {

	var issues []ProjectIssue
	for _, issue := range ghIssues {

		project, err := getProjectFromIssueBody(issue.GetBody())
		if err != nil {
			log.Warn().Msgf("Failed to get project name from issue metadata. Issue: %s", issue.GetTitle())
			continue
		}

		if project == nil {
			log.Debug().Msgf("Project not found in issue metadata. Issue: %s", issue.GetTitle())
			continue
		}

		issues = append(issues, ProjectIssue{
			Project: project.Project,
			Issue:   *issue,
			Kind:    project.Kind,
		})

	}

	return issues
}

func getProjectFromIssueBody(body string) (*GHProject, error) {
	idx := strings.Index(body, issueBodyProjectNameStartKeyword)
	if idx == -1 {
		return nil, errors.New("project name not found")
	}
	idx += len(issueBodyProjectNameStartKeyword)
	endIdx := strings.Index(body[idx:], issueBodyProjectNameEndKeyword)
	if endIdx == -1 {
		return nil, errors.New("project name not found")
	}
	// format: <!--folder/project_name-->
	projectNameTag := body[idx : idx+endIdx]
	projectJson := strings.ReplaceAll(strings.ReplaceAll(projectNameTag, "<!--", ""), "-->", "")
	var project GHProject
	if err := json.Unmarshal([]byte(projectJson), &project); err != nil {
		log.Warn().Msgf("Failed to find project details from issue body. %v. Ignoring issue.", err)
		return nil, nil
	}

	return &project, nil
}
