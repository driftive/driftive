package summary

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed tests/expected_summary.md
var expected string

func TestGetSummaryIssueBody(t *testing.T) {
	summary := GithubSummary{
		DriftedProjects:     []string{"projs/project1", "projs/project2"},
		ErroredProjects:     []string{"projs/project3"},
		RateLimitedProjects: []string{"projs/project4"},
		LastAnalysisDate:    "2021-08-01T03:32:12Z",
	}
	result, err := getSummaryIssueBody(summary)
	if err != nil {
		t.Errorf("Error: %s", err)
	}
	if strings.Trim(*result, " \n") != strings.Trim(expected, " \n") {
		t.Errorf("\nExpected\n-----\n%s\n-----, got -----\n%s\n-----", expected, *result)
	}
}
