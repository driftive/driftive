package drift

import (
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"sync"
	"time"
)

type DriftDetector struct {
	RepoDir     string
	Projects    []models.TypedProject
	Concurrency int
	workerWg    sync.WaitGroup
	results     chan DriftProjectResult
	semaphore   chan struct{}
}

type DriftProjectResult struct {
	Project models.TypedProject `json:"project"`
	Drifted bool                `json:"drifted"`
	// Succeeded true if the drift analysis succeeded, even if the project had drifted.
	Succeeded  bool   `json:"succeeded"`
	InitOutput string `json:"init_output"`
	PlanOutput string `json:"plan_output"`
}

type DriftDetectionResult struct {
	ProjectResults []DriftProjectResult `json:"project_results"`
	TotalDrifted   int                  `json:"total_drifted"`
	TotalProjects  int                  `json:"total_projects"`
	TotalChecked   int                  `json:"total_checked"`
	Duration       time.Duration        `json:"duration"`
}

func NewDriftDetector(repoDir string, projects []models.TypedProject, concurrency int) DriftDetector {
	return DriftDetector{
		RepoDir:     repoDir,
		Projects:    projects,
		Concurrency: concurrency,
		workerWg:    sync.WaitGroup{},
		results:     nil,
		semaphore:   make(chan struct{}, utils.Max(1, concurrency)),
	}
}
