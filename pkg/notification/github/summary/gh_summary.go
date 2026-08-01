package summary

import (
	"bytes"
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/drift"
	driftiveGithub "driftive/pkg/notification/github/types"
	"driftive/pkg/notification/report"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v88/github"
	"github.com/rs/zerolog/log"
	"sort"
	"strings"
	"text/template"
	"time"
)

//go:embed template/gh-summary-description.md
var summaryTemplate string

// SummaryProject is one row of one summary table, and is also what the hidden state block
// persists.
type SummaryProject struct {
	Dir string `json:"dir"`
	// IssueNumber is the GitHub issue tracking this project, or 0 when none exists.
	IssueNumber int `json:"issue_number,omitempty"`
	// RateLimited is true when an issue was wanted but max_open_issues blocked creation.
	RateLimited bool `json:"rate_limited,omitempty"`
	// FailedPhase is drift.PhaseInit or drift.PhasePlan; set only on errored projects.
	FailedPhase string `json:"failed_phase,omitempty"`
}

// DirCell renders the Project column. GFM splits table rows on "|" before inline parsing, so a
// pipe in a directory name has to be escaped even inside a code span.
func (p SummaryProject) DirCell() string {
	return "`" + strings.ReplaceAll(p.Dir, "|", "\\|") + "`"
}

// IssueLink renders the Issue column.
func (p SummaryProject) IssueLink() string {
	if p.IssueNumber > 0 {
		return fmt.Sprintf("[#%d](../issues/%d)", p.IssueNumber, p.IssueNumber)
	}
	if p.RateLimited {
		return "— _rate limited_"
	}
	return "—"
}

type GithubSummary struct {
	TotalProjects int `json:"total_projects"`
	NumDrifted    int `json:"num_drifted"`
	NumErrored    int `json:"num_errored"`
	NumSkipped    int `json:"num_skipped"`
	NumClean      int `json:"num_clean"`
	NumNotChecked int `json:"num_not_checked,omitempty"`

	Drifted []SummaryProject `json:"drifted,omitempty"`
	Errored []SummaryProject `json:"errored,omitempty"`
	Skipped []SummaryProject `json:"skipped,omitempty"`
	// OtherIssues are open driftive issues not represented above: the project was not part of
	// this run, or it came back clean while close_resolved is off.
	OtherIssues []SummaryProject `json:"other_issues,omitempty"`

	LastAnalysisDate    string `json:"last_analysis_date"`
	LastAnalysisDisplay string `json:"-"`
	Duration            string `json:"duration"`
	DashboardURL        string `json:"dashboard_url,omitempty"`
}

func (s GithubSummary) HasFindings() bool {
	return len(s.Drifted) > 0 || len(s.Errored) > 0
}

func (s GithubSummary) HasRateLimited() bool {
	for _, group := range [][]SummaryProject{s.Drifted, s.Errored} {
		for _, p := range group {
			if p.RateLimited {
				return true
			}
		}
	}
	return false
}

type GithubSummaryHandler struct {
	repoConfig   *repo.DriftiveRepoConfig
	config       *config.DriftiveConfig
	ghClient     *github.Client
	dashboardURL string
}

func NewGithubSummaryHandler(
	config *config.DriftiveConfig,
	repoConfig *repo.DriftiveRepoConfig,
	dashboardURL string) (*GithubSummaryHandler, error) {
	ghClient, err := github.NewClient(github.WithAuthToken(config.GithubToken))
	if err != nil {
		return nil, err
	}
	return &GithubSummaryHandler{
		config:       config,
		repoConfig:   repoConfig,
		ghClient:     ghClient,
		dashboardURL: dashboardURL,
	}, nil
}

// buildSummary converts a run's results plus the post-run issue state into the summary model.
// The tables are driven by the run, with issue numbers joined in by project dir; open issues
// with no matching result land in OtherIssues rather than disappearing.
func buildSummary(
	driftResult drift.DriftDetectionResult,
	state *driftiveGithub.GithubState,
	dashboardURL string,
	now time.Time,
) GithubSummary {
	classified := report.Classify(driftResult)

	driftIssues := state.IssueNumbersByDir(driftiveGithub.DriftIssueKind)
	errorIssues := state.IssueNumbersByDir(driftiveGithub.ErrorIssueKind)

	var rateLimitedDrifts, rateLimitedErrors []string
	if state != nil {
		rateLimitedDrifts = state.RateLimitedDrifts
		rateLimitedErrors = state.RateLimitedErrors
	}

	summary := GithubSummary{
		TotalProjects:       classified.TotalProjects,
		NumDrifted:          classified.NumDrifted(),
		NumErrored:          classified.NumErrored(),
		NumSkipped:          classified.NumSkipped(),
		NumClean:            classified.NumClean(),
		NumNotChecked:       classified.NotChecked,
		Drifted:             toRows(classified.Drifted, driftIssues, rateLimitedDrifts),
		Errored:             toRows(classified.Errored, errorIssues, rateLimitedErrors),
		Skipped:             toRows(classified.Skipped, nil, nil),
		LastAnalysisDate:    now.Format(time.RFC3339),
		LastAnalysisDisplay: now.UTC().Format("2006-01-02 15:04 UTC"),
		Duration:            classified.DurationText(),
		DashboardURL:        dashboardURL,
	}
	summary.OtherIssues = otherIssues(state, summary.Drifted, summary.Errored)

	return summary
}

func toRows(projects []report.Project, issues map[string]int, rateLimited []string) []SummaryProject {
	if len(projects) == 0 {
		return nil
	}
	limited := make(map[string]bool, len(rateLimited))
	for _, dir := range rateLimited {
		limited[dir] = true
	}

	rows := make([]SummaryProject, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, SummaryProject{
			Dir:         p.Dir,
			IssueNumber: issues[p.Dir],
			RateLimited: limited[p.Dir],
			FailedPhase: p.FailedPhase,
		})
	}
	return rows
}

func otherIssues(state *driftiveGithub.GithubState, reported ...[]SummaryProject) []SummaryProject {
	if state == nil {
		return nil
	}

	seen := make(map[string]bool)
	for _, group := range reported {
		for _, p := range group {
			seen[p.Dir] = true
		}
	}

	var rows []SummaryProject
	for _, group := range [][]driftiveGithub.ProjectIssue{state.DriftIssuesOpen, state.ErrorIssuesOpen} {
		for _, issue := range group {
			if seen[issue.Project.Dir] {
				continue
			}
			seen[issue.Project.Dir] = true
			rows = append(rows, SummaryProject{Dir: issue.Project.Dir, IssueNumber: issue.Issue.Number})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Dir < rows[j].Dir })
	return rows
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

func (g *GithubSummaryHandler) UpdateSummary(ctx context.Context, driftResult drift.DriftDetectionResult, state *driftiveGithub.GithubState) {
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

	summary := buildSummary(driftResult, state, g.dashboardURL, time.Now())

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
