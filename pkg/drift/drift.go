package drift

import (
	"driftive/pkg/exec"
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"github.com/rs/zerolog/log"
	"strings"
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
	Project   string
	Drifted   bool
	Succeeded bool
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

func (d *DriftDetector) detectDriftConcurrently(project models.Project, projectDir string) {
	defer func() {
		<-d.semaphore
	}()
	defer d.workerWg.Done()
	result, err := d.detectDrift(project)
	if err != nil {
		log.Info().Msgf("Error checking drift in %s: %v", project.Dir, err)
	}
	if result {
		log.Info().Msgf("Drift detected in project %s", projectDir)
	}
	d.results <- DriftProjectResult{Project: projectDir, Drifted: result}
}

func (d *DriftDetector) DetectDrift() DriftDetectionResult {
	log.Info().Msgf("Starting drift analysis in %s. Concurrency: %d", d.RepoDir, d.Concurrency)
	d.results = make(chan DriftProjectResult, len(d.Projects))
	var totalChecked = 0
	startTime := time.Now()

	for idx, proj := range d.Projects {
		projectDir := strings.TrimPrefix(strings.Replace(proj.Dir, d.RepoDir, "", -1), utils.PathSeparator)

		if projectDir == "" {
			continue
		}

		totalChecked++
		log.Info().Msgf("Checking drift in project %d/%d: %s", idx+1, len(d.Projects), projectDir)
		d.workerWg.Add(1)
		d.semaphore <- struct{}{}
		go d.detectDriftConcurrently(proj, projectDir)
	}

	d.workerWg.Wait()
	close(d.results)

	driftedProjects := make([]DriftProjectResult, 0)
	for result := range d.results {
		if result.Drifted {
			driftedProjects = append(driftedProjects, result)
		}
	}

	result := DriftDetectionResult{
		DriftedProjects: driftedProjects,
		TotalDrifted:    len(driftedProjects),
		TotalProjects:   len(d.Projects),
		TotalChecked:    len(d.Projects),
		Duration:        time.Since(startTime),
	}
	return result
}

func (d *DriftDetector) detectDrift(project models.Project) (bool, error) {
	executor := exec.NewExecutor(project.Dir, project.Type)
	result, err := executor.Init("-upgrade", "-lock=false")
	if err != nil {
		log.Info().Msgf("Error running init command in %s: %v", project.Dir, err)
		log.Info().Msg(result)
		return false, err
	}
	result, err = executor.Plan("-lock=false")
	if err != nil {
		log.Info().Msgf("Error running plan command in %s: %v", project.Dir, err)
		log.Info().Msg(result)
		return false, err
	}
	return d.isDriftDetected(result), nil
}

func (d *DriftDetector) isDriftDetected(commandOutput string) bool {
	noChangesPatterns := []string{"Your infrastructure matches the configuration", "No changes. Infrastructure is up-to-date."}
	for _, pattern := range noChangesPatterns {
		if strings.Contains(commandOutput, pattern) {
			return false
		}
	}
	return true
}
