package summary

import (
	"bytes"
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	driftiveGithub "driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog/log"
	"strings"
	"text/template"
	"time"
)

//go:embed template/gh-summary-description.md
var summaryTemplate string

type GithubSummary struct {
	RateLimitedProjects []string                      `json:"rate_limited_projects"`
	DriftedProjects     []driftiveGithub.ProjectIssue `json:"drifted_projects"`
	ErroredProjects     []driftiveGithub.ProjectIssue `json:"errored_projects"`
	LastAnalysisDate    string                        `json:"last_analysis_date"`
}

type GithubSummaryHandler struct {
	repoConfig    *repo.DriftiveRepoConfig
	config        *config.DriftiveConfig
	ghClient      *github.Client
	allOpenIssues []*vcstypes.VCSIssue
}

func NewGithubSummaryHandler(
	config *config.DriftiveConfig,
	repoConfig *repo.DriftiveRepoConfig,
	allOpenIssues []*vcstypes.VCSIssue) *GithubSummaryHandler {
	ghClient := github.NewClient(nil).WithAuthToken(config.GithubToken)
	return &GithubSummaryHandler{
		config:        config,
		repoConfig:    repoConfig,
		ghClient:      ghClient,
		allOpenIssues: allOpenIssues,
	}
}

func getSummaryIssueBody(summary GithubSummary) (*string, error) {
	tmpl, err := template.New("gh-summary").Parse(strings.Trim(summaryTemplate, " \n"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse github issue description template")
		return nil, err
	}

	jsonBytes, err := json.Marshal(summary)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal github summary")
		return nil, err
	}

	encodedJsonString := string(jsonBytes)

	templateArgs := struct {
		GithubSummary
		State string
	}{
		GithubSummary: summary,
		State:         encodedJsonString,
	}
	buff := new(bytes.Buffer)
	err = tmpl.Execute(buff, templateArgs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute github issue description template")
		return nil, err
	}

	buffString := buff.String()
	return &buffString, nil
}

func (g *GithubSummaryHandler) listAllIssues(ctx context.Context) ([]*github.Issue, error) {
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
		opt.ListOptions.Page = resp.NextPage
	}

	return openIssues, nil
}

func (g *GithubSummaryHandler) UpdateSummary(ctx context.Context, state *driftiveGithub.GithubState) {
	log.Info().Msg("Updating Github summary issue...")
	// Split owner/repository_name
	ownerRepo := strings.Split(g.config.GithubContext.Repository, "/")
	if len(ownerRepo) != 2 {
		log.Error().Msg("Invalid repository name in GithubContext")
		return
	}

	issues, err := g.listAllIssues(ctx)
	if err != nil {
		log.Error().Msgf("Failed to get open issues. %v", err)
		return
	}

	var summaryIssue *github.Issue
	for _, issue := range issues {
		if *issue.Title == g.repoConfig.GitHub.Summary.IssueTitle {
			summaryIssue = issue
			break
		}
	}

	summary := GithubSummary{
		RateLimitedProjects: state.RateLimitedDrifts,
		DriftedProjects:     state.DriftIssuesOpen,
		ErroredProjects:     state.ErrorIssuesOpen,
		LastAnalysisDate:    time.Now().Format(time.RFC3339),
	}

	issueBody, err := getSummaryIssueBody(summary)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get summary issue body")
		return
	}

	if summaryIssue != nil {
		_, _, err = g.ghClient.Issues.Edit(ctx,
			ownerRepo[0],
			ownerRepo[1],
			*summaryIssue.Number,
			&github.IssueRequest{
				Body: issueBody,
			})
		if err != nil {
			log.Error().Err(err).Msg("Failed to update summary issue")
		}
	} else {
		_, _, err = g.ghClient.Issues.Create(ctx,
			ownerRepo[0],
			ownerRepo[1],
			&github.IssueRequest{
				Title: &g.repoConfig.GitHub.Summary.IssueTitle,
				Body:  issueBody,
			})
		if err != nil {
			log.Error().Err(err).Msg("Failed to create summary issue")
		}
	}
	log.Info().Msg("Github summary issue updated")
}
