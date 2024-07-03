package drift

import (
	"driftive/pkg/config"
	"driftive/pkg/exec"
	"driftive/pkg/utils"
	"github.com/rs/zerolog/log"
	"strings"
	"sync"
	"time"
)

type DriftDetector struct {
	RepoDir     string
	Projects    []config.Project
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

func NewDriftDetector(repoDir string, projects []config.Project, concurrency int) DriftDetector {
	return DriftDetector{
		RepoDir:     repoDir,
		Projects:    projects,
		Concurrency: concurrency,
		workerWg:    sync.WaitGroup{},
		results:     nil,
		semaphore:   make(chan struct{}, utils.Max(1, concurrency)),
	}
}

func (d *DriftDetector) detectDriftConcurrently(dir string, projectDir string) {
	defer func() {
		<-d.semaphore
	}()
	defer d.workerWg.Done()
	result, err := d.detectDrift(dir)
	if err != nil {
		log.Info().Msgf("Error checking drift in %s: %v", dir, err)
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
		projectDir := strings.TrimPrefix(strings.Replace(proj.Dir, d.RepoDir, "", -1), "/")

		if projectDir == "" {
			continue
		}

		totalChecked++
		log.Info().Msgf("Checking drift in project %d/%d: %s", idx+1, len(d.Projects), projectDir)
		d.workerWg.Add(1)
		d.semaphore <- struct{}{}
		go d.detectDriftConcurrently(proj.Dir, projectDir)
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

func (d *DriftDetector) detectDrift(dir string) (bool, error) {
	result, err := exec.RunCommandInDir(dir, "terragrunt", "init", "-upgrade", "-lock=false")
	if err != nil {
		log.Info().Msgf("Error running init command in %s: %v", dir, err)
		log.Info().Msg(result)
		return false, err
	}
	result, err = exec.RunCommandInDir(dir, "terragrunt", "plan", "-lock=false")
	if err != nil {
		log.Info().Msgf("Error running plan command in %s: %v", dir, err)
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
