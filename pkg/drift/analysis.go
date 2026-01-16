package drift

import (
	"context"
	"driftive/pkg/exec"
	"driftive/pkg/models"
	"driftive/pkg/utils"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (d *DriftDetector) detectDriftConcurrently(project models.TypedProject, projectDir string) {
	defer func() {
		<-d.semaphore
	}()
	defer d.workerWg.Done()
	result, err := d.detectDrift(project)
	if err != nil {
		log.Info().Msgf("Error checking drift in %s: %v", project.Dir, err)
	}
	if result.Drifted {
		log.Info().Msgf("Drift detected in project %s", projectDir)
	}
	d.results <- result
}

func (d *DriftDetector) DetectDrift(ctx context.Context) DriftDetectionResult {
	absolutePath, err := filepath.Abs(d.RepoDir)
	if err != nil {
		log.Error().Msgf("Error getting absolute path of %s: %v", d.RepoDir, err)
		return DriftDetectionResult{}
	}

	log.Info().Msgf("Starting drift analysis in %s. Concurrency: %d", absolutePath, d.Config.Concurrency)
	d.results = make(chan DriftProjectResult, len(d.Projects))
	var totalChecked = 0
	startTime := time.Now()

	for idx, proj := range d.Projects {
		projectDir := strings.TrimPrefix(strings.ReplaceAll(proj.Dir, d.RepoDir, ""), utils.PathSeparator)

		if projectDir == "" {
			continue
		}

		totalChecked++
		log.Info().Msgf("Checking drift in project %d/%d: %s (%s)", idx+1, len(d.Projects), projectDir, models.ProjectTypeToStr(proj.Type))
		d.workerWg.Add(1)
		d.semaphore <- struct{}{}
		go d.detectDriftConcurrently(proj, projectDir)
	}

	d.workerWg.Wait()
	close(d.results)

	projectResults := make([]DriftProjectResult, 0)
	driftedCount := 0
	erroredCount := 0
	for result := range d.results {
		projectResults = append(projectResults, result)
		if result.Drifted {
			driftedCount++
		}
		if !result.Succeeded {
			erroredCount++
		}
	}

	result := DriftDetectionResult{
		ProjectResults: projectResults,
		TotalDrifted:   driftedCount,
		TotalErrored:   erroredCount,
		TotalProjects:  len(d.Projects),
		TotalChecked:   len(d.Projects),
		Duration:       time.Since(startTime),
	}

	if d.RepoConfig.Settings.SkipIfOpenPR {
		d.handleSkipIfContainsPRChanges(&result)
	}

	return result
}

func (d *DriftDetector) detectDrift(project models.TypedProject) (DriftProjectResult, error) {
	executor := exec.NewExecutor(project.Dir, project.Type)
	output, err := executor.Init("-upgrade", "-lock=false", "-no-color")

	if err != nil {
		log.Info().Msgf("Error running init command in %s: %v", project.Dir, err)
		log.Info().Msg(output)
		return DriftProjectResult{Project: project, Drifted: false, Succeeded: false, InitOutput: output, PlanOutput: ""}, err
	}
	output, err = executor.Plan("-lock=false", "-no-color")
	if err != nil {
		log.Info().Msgf("Error running plan command in %s: %v", project.Dir, err)
		log.Info().Msg(output)
		return DriftProjectResult{Project: project, Drifted: false, Succeeded: false, InitOutput: "", PlanOutput: executor.ParseErrorOutput(output)}, err
	}
	driftDetected := d.isDriftDetected(output)
	if driftDetected {
		output = executor.ParsePlan(output)
	}
	result := DriftProjectResult{Project: project, Drifted: driftDetected, Succeeded: true, InitOutput: "", PlanOutput: output}
	return result, nil
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
