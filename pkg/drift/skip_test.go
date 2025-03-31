package drift

import (
	"driftive/pkg/models"
	"driftive/pkg/vcs/vcstypes"
	"strconv"
	"testing"
	"time"
)

func makeMockedResult(total, drifted int) DriftDetectionResult {

	projs := make([]DriftProjectResult, 0)

	for i := 0; i < total; i++ {

		var isDrifted bool
		if i < drifted {
			isDrifted = true
		} else {
			isDrifted = false
		}

		p := DriftProjectResult{
			Project: models.TypedProject{
				Dir:  "gcp/myproject/app" + strconv.Itoa(i+1),
				Type: models.Terragrunt,
			},
			Drifted:        isDrifted,
			Succeeded:      true,
			InitOutput:     "FakeInitOutput",
			PlanOutput:     "FakePlanOutput",
			SkippedDueToPR: false,
		}
		projs = append(projs, p)
	}

	result := DriftDetectionResult{
		ProjectResults: projs,
		TotalDrifted:   drifted,
		TotalProjects:  total,
		TotalChecked:   total,
		Duration:       5 * time.Minute,
	}
	return result
}

func TestPRSkip(t *testing.T) {
	result := makeMockedResult(4, 3)
	detector := DriftDetector{
		Stash: Stash{
			OpenPRChangedFiles: []string{
				"gcp/myproject/app1/main.tf",
				"gcp/myproject/app1/something.tf",

				"gcp/myproject/app2/main.tf",
			},
			OpenIssues: make([]*vcstypes.VCSIssue, 0),
		},
	}
	detector.handleSkipIfContainsPRChanges(&result)

	totalSkipped := 0
	for _, projectResult := range result.ProjectResults {
		if projectResult.SkippedDueToPR {
			totalSkipped++
		}
	}

	if totalSkipped != 2 {
		t.Errorf("Skipped %d project(s) but expected 2", totalSkipped)
	}
}
