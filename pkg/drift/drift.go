package drift

import (
	"driftive/pkg/exec"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DriftDetector struct {
	RepoDir     string
	Concurrency int
}

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

func NewDriftDetector(repoDir string, concurrency int) DriftDetector {
	return DriftDetector{RepoDir: repoDir, Concurrency: concurrency}
}

func (d DriftDetector) detectDriftConcurrently(sem chan struct{}, wg *sync.WaitGroup, results chan DriftProjectResult, dir string, projectDir string) {
	defer func() {
		<-sem
	}()
	defer wg.Done()
	result, err := d.detectDrift(dir)
	if err != nil {
		println(fmt.Sprintf("Error checking drift in %s: %v", dir, err))
	}
	if result {
		println(fmt.Sprintf("Drift detected in project %s", projectDir))
		results <- DriftProjectResult{Project: projectDir, Drifted: result}
	}
}

func (d DriftDetector) DetectDrift() DriftDetectionResult {

	println(fmt.Sprintf("Starting drift analysis in %s. Concurrency: %d", d.RepoDir, d.Concurrency))

	projectDirs := d.DetectTerragruntProjects(d.RepoDir)
	println(fmt.Sprintf("Detected %d projects", len(projectDirs)))

	var driftedProjects []DriftProjectResult
	var totalProjects = len(projectDirs)
	var totalChecked = 0

	sem := make(chan struct{}, d.Concurrency)
	wg := sync.WaitGroup{}
	results := make(chan DriftProjectResult, totalProjects)
	startTime := time.Now()

	for idx, dir := range projectDirs {
		projectDir := strings.TrimPrefix(strings.Replace(dir, d.RepoDir, "", -1), "/")

		if projectDir == "" {
			continue
		}

		totalChecked++
		println(fmt.Sprintf("Checking drift in project %d/%d: %s", idx+1, totalProjects, projectDir))
		wg.Add(1)
		sem <- struct{}{}
		go d.detectDriftConcurrently(sem, &wg, results, dir, projectDir)
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

func (d DriftDetector) detectDrift(dir string) (bool, error) {
	result, err := exec.RunCommandInDir(dir, "terragrunt", "init", "-upgrade", "-lock=false")
	if err != nil {
		println(fmt.Sprintf("Error running init command in %s: %v", dir, err))
		return false, err
	}
	result, err = exec.RunCommandInDir(dir, "terragrunt", "plan", "-lock=false")
	if err != nil {
		return false, err
	}
	return d.isDriftDetected(result), nil
}

func (d DriftDetector) isDriftDetected(commandOutput string) bool {
	noChangesPatterns := []string{"Your infrastructure matches the configuration", "No changes. Infrastructure is up-to-date."}
	for _, pattern := range noChangesPatterns {
		if strings.Contains(commandOutput, pattern) {
			return false
		}
	}
	return true
}

func (d DriftDetector) isPartOfCacheFolder(dir string) bool {
	return strings.Contains(dir, ".terragrunt-cache")
}

// DetectTerragruntProjects detects all terragrunt projects recursively in a directory
func (d DriftDetector) DetectTerragruntProjects(dir string) []string {
	targetFileName := "terragrunt.hcl"
	var foldersContainingFile []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the file is a terragrunt file. Ignore root terragrunt files.
		if !info.IsDir() && info.Name() == targetFileName && path != filepath.Join(dir, targetFileName) && !d.isPartOfCacheFolder(path) {
			folder := filepath.Dir(path)
			foldersContainingFile = append(foldersContainingFile, folder)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %v: %v\n", dir, err)
		return nil
	}

	return foldersContainingFile
}
