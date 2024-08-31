package github

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"strings"
	"text/template"
)

//go:embed template/gh-summary-description.md
var summaryTemplate string

type GithubSummary struct {
	RateLimitedProjects []string `json:"rate_limited_projects"`
	DriftedProjects     []string `json:"drifted_projects"`
	ErroredProjects     []string `json:"errored_projects"`
	LastAnalysisDate    string   `json:"last_analysis_date"`
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

func UpdateSummary(summary GithubSummary) {

}
