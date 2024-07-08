package drift

import (
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"sync"
	"time"
)

type DriftDetector struct {
	RepoDir     string
	Projects    []models.Project
	Concurrency int
	workerWg    sync.WaitGroup
	results     chan DriftProjectResult
	semaphore   chan struct{}
}

type DriftProjectResult struct {
	Project models.Project
	Drifted bool
	// Succeeded is true if the drift detection process succeeded, even if the project has drifted or not.
	Succeeded  bool
	InitOutput string
	PlanOutput string
}

type DriftDetectionResult struct {
	DriftedProjects []DriftProjectResult
	TotalDrifted    int
	TotalProjects   int
	TotalChecked    int
	Duration        time.Duration
}

func NewDriftDetector(repoDir string, projects []models.Project, concurrency int) DriftDetector {
	return DriftDetector{
		RepoDir:     repoDir,
		Projects:    projects,
		Concurrency: concurrency,
		workerWg:    sync.WaitGroup{},
		results:     nil,
		semaphore:   make(chan struct{}, utils.Max(1, concurrency)),
	}
}
