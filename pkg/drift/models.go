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
	Project models.TypedProject
	Drifted bool
	// Succeeded true if the drift analysis succeeded, even if the project had drifted.
	Succeeded  bool
	InitOutput string
	PlanOutput string
}

type DriftDetectionResult struct {
	ProjectResults []DriftProjectResult
	TotalDrifted   int
	TotalProjects  int
	TotalChecked   int
	Duration       time.Duration
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
