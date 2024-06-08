package pkg

import (
	"drifter/pkg/exec"
	"fmt"
	"strings"
	"sync"
	"time"
)

type DriftProjectResult struct {
	Project string
	Drifted bool
}

type DriftDetectionResult struct {
	DriftedProjects []DriftProjectResult
	TotalDrifted    int
	TotalProjects   int
	TotalChecked    int
	Duration        time.Duration
}

func detectDriftConcurrently(sem chan struct{}, wg *sync.WaitGroup, results chan DriftProjectResult, dir string, projectDir string) {
	defer func() {
		<-sem
	}()
	defer wg.Done()
	result, err := detectDrift(dir)
	if err != nil {
		println(fmt.Sprintf("Error checking drift in %s: %v", dir, err))
	}
	if result {
		println(fmt.Sprintf("Drift detected in project %s", projectDir))
		results <- DriftProjectResult{Project: projectDir, Drifted: result}
	}
}

func DetectDrift(repoDir string, dirs []string) DriftDetectionResult {

	sem := make(chan struct{}, 8)
	wg := sync.WaitGroup{}
	results := make(chan DriftProjectResult, len(dirs))
	startTime := time.Now()

	var driftedProjects []DriftProjectResult
	var totalProjects = len(dirs)
	var totalChecked = 0

	for idx, dir := range dirs {
		projectDir := strings.TrimPrefix(strings.Replace(dir, repoDir, "", -1), "/")

		if projectDir == "" {
			continue
		}

		totalChecked++
		println(fmt.Sprintf("Checking drift in project %d/%d: %s", idx+1, len(dirs), projectDir))
		wg.Add(1)
		sem <- struct{}{}
		go detectDriftConcurrently(sem, &wg, results, dir, projectDir)
	}

	wg.Wait()
	close(results)

	for result := range results {
		driftedProjects = append(driftedProjects, result)
	}

	result := DriftDetectionResult{
		DriftedProjects: driftedProjects,
		TotalDrifted:    len(driftedProjects),
		TotalProjects:   totalProjects,
		TotalChecked:    totalProjects,
		Duration:        time.Since(startTime),
	}
	return result
}

func detectDrift(dir string) (bool, error) {
	result, err := exec.RunCommandInDir(dir, "terragrunt", "plan", "-lock=false")
	if err != nil {
		return false, err
	}
	return isDriftDetected(result), nil
}

func isDriftDetected(terragruntOutput string) bool {
	noChangesPatterns := []string{"Your infrastructure matches the configuration", "No changes. Infrastructure is up-to-date."}
	for _, pattern := range noChangesPatterns {
		if strings.Contains(terragruntOutput, pattern) {
			return false
		}
	}
	return true
}
