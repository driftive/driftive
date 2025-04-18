package summary

import (
	"driftive/pkg/models"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
	_ "embed"
	"strings"
	"testing"
)

//go:embed tests/expected_summary.md
var expected string

func TestGetSummaryIssueBody(t *testing.T) {
	summary := GithubSummary{
		DriftedProjects: []types.ProjectIssue{{
			Project: models.Project{
				Dir: "projs/project1",
			},
			Issue: vcstypes.VCSIssue{
				Number: 1,
			},
			Kind: "drift",
		}, {
			Project: models.Project{
				Dir: "projs/project2",
			},
			Issue: vcstypes.VCSIssue{
				Number: 2,
			},
			Kind: "drift",
		}},
		ErroredProjects: []types.ProjectIssue{{
			Project: models.Project{
				Dir: "projs/project3",
			},
			Issue: vcstypes.VCSIssue{
				Number: 3,
			},
			Kind: "drift",
		}},
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
