package github

import (
	"bytes"
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"driftive/pkg/notification/github/summary"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/utils"
	"driftive/pkg/utils/ghutils"
	"driftive/pkg/vcs"
	"driftive/pkg/vcs/vcstypes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog/log"
)

const (
	issueTitleFormat                 = "drift detected: %s"
	errorIssueTitleFormat            = "plan error: %s"
	maxIssueBodySize                 = 64000 // Lower than 65535 to account for other metadata
	ErrRepoNotProvided               = "repository or owner not provided"
	issueBodyProjectNameStartKeyword = "<!--PROJECT_JSON_START-->"
	issueBodyProjectNameEndKeyword   = "<!--PROJECT_JSON_END-->"
	ErrIssueMetadataNotFound         = "issue_metadata_not_found"
)

//go:embed template/gh-issue-description.md
var issueBodyTemplate string

//go:embed template/gh-error-issue-description.md
var errorIssueBodyTemplate string

type GithubIssueNotification struct {
	config     *config.DriftiveConfig
	repoConfig *repo.DriftiveRepoConfig
	ghClient   *github.Client
	scm        vcs.VCS
}

func NewGithubIssueNotification(config *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig, ghOpts vcs.VCS) (*GithubIssueNotification, error) {
	if config.GithubContext.Repository == "" || config.GithubContext.RepositoryOwner == "" {
		log.Warn().Msg("Github repository or owner not provided. Skipping github notification")
		return nil, errors.New(ErrRepoNotProvided)
	}

	ghClient, err := ghutils.GitHubClient(config.GithubToken)
	if err != nil {
		log.Warn().Msg("Github token not provided. Skipping github notification")
		return nil, err
	}
	return &GithubIssueNotification{config: config, repoConfig: repoConfig, ghClient: ghClient, scm: ghOpts}, nil
}

func parseGithubBodyTemplate(project drift.DriftProjectResult, bodyTemplate string) (*string, error) {
	projectKind := types.DriftIssueKind
	if !project.Drifted && !project.Succeeded {
		projectKind = types.ErrorIssueKind
	}

	ghProject := types.GHProject{
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

func (g *GithubIssueNotification) Handle(ctx context.Context, analysisResult drift.DriftDetectionResult) (*types.GithubState, error) {
	allOpenIssues, err := g.scm.GetAllOpenRepoIssues(ctx)
	if err != nil {
		log.Error().Msgf("Failed to get open issues. %v", err)
		return nil, err
	}

	state, err := g.HandleIssues(ctx, analysisResult, allOpenIssues)
	if err != nil {
		log.Error().Msgf("Failed to update github issues. %v", err)
		return nil, err
	}

	log.Info().Msgf("Github issues updated")
	if g.repoConfig.GitHub.Summary.Enabled {
		summary.NewGithubSummaryHandler(g.config, g.repoConfig, allOpenIssues).UpdateSummary(ctx, state)
	} else {
		log.Info().Msg("Github summary is disabled. Skipping summary update")
	}

	return state, nil
}

func (g *GithubIssueNotification) HandleIssues(ctx context.Context,
	driftResult drift.DriftDetectionResult,
	allOpenIssues []*vcstypes.VCSIssue) (*types.GithubState, error) {
	allDriftiveOpenIssues := getProjectIssuesFromGHIssueBodies(allOpenIssues)
	numOpenDriftIssues := 0
	for _, issue := range allDriftiveOpenIssues {
		if issue.Kind == types.DriftIssueKind {
			numOpenDriftIssues++
		}
	}
	numOpenErrorIssues := 0
	for _, issue := range allDriftiveOpenIssues {
		if issue.Kind == types.ErrorIssueKind {
			numOpenErrorIssues++
		}
	}

	var closeableDriftIssues []types.ProjectIssue
	for _, project := range allDriftiveOpenIssues {
		if project.Kind == types.DriftIssueKind {
			for _, projectResult := range driftResult.ProjectResults {
				if !projectResult.Drifted && project.Project.Dir == projectResult.Project.Dir {
					closeableDriftIssues = append(closeableDriftIssues, project)
				}
			}
		}
	}

	var closeableErrorIssues []types.ProjectIssue
	for _, project := range allDriftiveOpenIssues {
		if project.Kind == types.ErrorIssueKind {
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
	var newlyCreatedIssues []types.ProjectIssue
	var rateLimitedProjectDirs []string

	// Create issues for drifted projects
	for _, projectResult := range driftResult.ProjectResults {
		if projectResult.Drifted && !projectResult.SkippedDueToPR {
			issueBody, err := parseGithubBodyTemplate(projectResult, issueBodyTemplate)
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse github issue description template")
				continue
			}

			issue := types.GithubIssue{
				Title:   fmt.Sprintf(issueTitleFormat, projectResult.Project.Dir),
				Body:    *issueBody,
				Labels:  g.repoConfig.GitHub.Issues.Labels,
				Project: projectResult.Project,
				Kind:    types.DriftIssueKind,
			}
			createOrUpdateResult := g.scm.CreateOrUpdateIssue(
				ctx,
				issue,
				allOpenIssues,
				numOpenDriftIssues >= g.repoConfig.GitHub.Issues.MaxOpenIssues,
			)
			if createOrUpdateResult.Created {
				numOpenDriftIssues++
				newlyCreatedIssues = append(newlyCreatedIssues, types.ProjectIssue{
					Issue: *createOrUpdateResult.Issue,
					Project: models.Project{
						Dir: projectResult.Project.Dir,
					},
					Kind: types.DriftIssueKind,
				})
			}
			if createOrUpdateResult.RateLimited {
				rateLimitedProjectDirs = append(rateLimitedProjectDirs, projectResult.Project.Dir)
			}
		} else if projectResult.Drifted && projectResult.SkippedDueToPR {
			log.Info().Msgf("Skipping drift notification for %s due to open PRs", projectResult.Project.Dir)
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

				issue := types.GithubIssue{
					Title:   fmt.Sprintf(errorIssueTitleFormat, projectResult.Project.Dir),
					Body:    *issueBody,
					Labels:  g.repoConfig.GitHub.Issues.Errors.Labels,
					Project: projectResult.Project,
					Kind:    types.ErrorIssueKind,
				}
				createOrUpdateResult := g.scm.CreateOrUpdateIssue(
					ctx,
					issue,
					allOpenIssues,
					numOpenErrorIssues >= g.repoConfig.GitHub.Issues.Errors.MaxOpenIssues,
				)
				if createOrUpdateResult.Created {
					numOpenErrorIssues++
					newlyCreatedIssues = append(newlyCreatedIssues, types.ProjectIssue{
						Issue: *createOrUpdateResult.Issue,
						Project: models.Project{
							Dir: projectResult.Project.Dir,
						},
						Kind: types.ErrorIssueKind,
					})
				}
			}
		}
	}

	currentOpenIssues := append(allDriftiveOpenIssues, newlyCreatedIssues...)
	currentDriftedIssues := filterIssues(filterIssuesByKind(currentOpenIssues, types.DriftIssueKind), closedDriftIssues)
	currentErroredIssues := filterIssues(filterIssuesByKind(currentOpenIssues, types.ErrorIssueKind), closedErrorIssues)

	return &types.GithubState{
		RateLimitedDrifts:   rateLimitedProjectDirs,
		DriftIssuesOpen:     currentDriftedIssues,
		DriftIssuesResolved: closedDriftIssues,
		ErrorIssuesOpen:     currentErroredIssues,
		ErrorIssuesResolved: closedErrorIssues,
	}, nil
}

func filterIssuesByKind(allIssues []types.ProjectIssue, kind string) []types.ProjectIssue {
	var issues []types.ProjectIssue
	for _, issue := range allIssues {
		if issue.Kind == kind {
			issues = append(issues, issue)
		}
	}
	return issues
}

// getProjectIssuesFromGHIssueBodies lists the issues that have any of the labels or the title contains the keyword
func getProjectIssuesFromGHIssueBodies(ghIssues []*vcstypes.VCSIssue) []types.ProjectIssue {
	issues := make([]types.ProjectIssue, 0)
	for _, issue := range ghIssues {
		project, err := getProjectFromIssueBody(issue.Body)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to get project name from issue metadata. Issue title: %s", issue.Title)
			continue
		}

		if project == nil {
			log.Debug().Msgf("Project not found in issue metadata. Issue: %s", issue.Title)
			continue
		}

		issues = append(issues, types.ProjectIssue{
			Project: project.Project,
			Issue:   *issue,
			Kind:    project.Kind,
		})
	}
	return issues
}

func getProjectFromIssueBody(body string) (*types.GHProject, error) {
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
	var project types.GHProject
	if err := json.Unmarshal([]byte(projectJson), &project); err != nil {
		log.Warn().Msgf("Failed to find project details from issue body. %v. Ignoring issue.", err)
		return nil, errors.New(ErrIssueMetadataNotFound)
	}

	return &project, nil
}

func (g *GithubIssueNotification) closeIssues(ctx context.Context, issues []types.ProjectIssue) []types.ProjectIssue {
	if !g.repoConfig.GitHub.Issues.CloseResolved && len(issues) > 0 {
		log.Warn().Msg("Note: There are GH drift issues but driftive is not configured to close them.")
		return []types.ProjectIssue{}
	}

	var closedIssues []types.ProjectIssue
	for _, projIssue := range issues {
		closed := g.CloseIssueWithComment(ctx, projIssue)
		if closed {
			closedIssues = append(closedIssues, projIssue)
		}
	}

	return closedIssues
}
