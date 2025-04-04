package drift

import (
	"driftive/pkg/config"
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
	repoDir := "/home/user/repo_dir/"
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
		Config: &config.DriftiveConfig{
			RepositoryPath: repoDir,
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

func TestProjectFolderIfRootWorkingDir(t *testing.T) {
	projectDir := "project1"
	file := "project1/main.tf"
	fileFolder := removeTrailingSlash(getFolder(file))
	if fileFolder != projectDir {
		t.Errorf("Expected 'project1' but got '%s'", fileFolder)
	}

	getFolderResult := getFolder("/home/user/repo/project1/main.tofu")
	if getFolderResult != "/home/user/repo/project1/" {
		t.Errorf("Expected '/home/user/repo/project1' but got '%s'", getFolderResult)
	}

	getFolderResult = getFolder("project1/main.tofu")
	if getFolderResult != "project1/" {
		t.Errorf("Expected 'project1' but got '%s'", getFolderResult)
	}

	result := removeTrailingSlash("project1")
	if result != "project1" {
		t.Errorf("Expected 'project1' but got '%s'", result)
	}
}

func TestRemoveTrailingSlash(t *testing.T) {
	path := "/home/user/repo_dir/"
	result := removeTrailingSlash(path)
	if result != "/home/user/repo_dir" {
		t.Errorf("Expected '/home/user/repo_dir' but got '%s'", result)
	}

	path = "/home/user/repo_dir"
	result = removeTrailingSlash(path)
	if result != "/home/user/repo_dir" {
		t.Errorf("Expected '/home/user/repo_dir' but got '%s'", result)
	}
}
