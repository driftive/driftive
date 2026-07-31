package drift

import (
	"context"
	"driftive/pkg/models"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (d *DriftDetector) detectDriftConcurrently(ctx context.Context, project models.TypedProject, projectDir string) {
	defer func() {
		<-d.semaphore
	}()
	defer d.workerWg.Done()

	// Reported from inside the worker, after the semaphore is acquired, so "running" reflects
	// actual concurrency rather than the whole backlog.
	if d.OnProjectStart != nil {
		d.OnProjectStart(projectDir)
	}

	result, err := d.detectDrift(ctx, project)
	if err != nil {
		log.Info().Msgf("Error checking drift in %s: %v", project.Dir, err)
	}
	if result.Drifted {
		log.Info().Msgf("Drift detected in project %s", projectDir)
	}
	// Report the repo-relative dir rather than the discovered path, which carries whatever
	// prefix --repo-path had (or the temp clone dir under --repo-url). Must happen after
	// detectDrift, which uses project.Dir as the subprocess working directory.
	result.Project.Dir = projectDir

	// After the dir is normalized, so both callbacks agree on it.
	if d.OnProjectDone != nil {
		d.OnProjectDone(result)
	}

	d.results <- result
}

// relativeProjectDir returns proj relative to the repository root. The repo root itself is
// reported as ".".
func relativeProjectDir(repoDir, projectDir string) string {
	rel, err := filepath.Rel(repoDir, projectDir)
	if err != nil {
		return projectDir
	}
	if rel == "" {
		return "."
	}
	return rel
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
		projectDir := relativeProjectDir(d.RepoDir, proj.Dir)

		// Honor cancellation between projects so an interrupted run doesn't keep
		// spinning up new terraform/tofu processes after Ctrl-C.
		if ctx.Err() != nil {
			log.Info().Msgf("Drift analysis cancelled after %d/%d projects: %v", idx, len(d.Projects), ctx.Err())
			break
		}

		totalChecked++
		log.Info().Msgf("Checking drift in project %d/%d: %s (%s)", idx+1, len(d.Projects), projectDir, models.ProjectTypeToStr(proj.Type))
		d.workerWg.Add(1)
		d.semaphore <- struct{}{}
		go d.detectDriftConcurrently(ctx, proj, projectDir)
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
		TotalChecked:   totalChecked,
		Duration:       time.Since(startTime),
	}

	if d.RepoConfig.Settings.SkipIfOpenPR {
		d.handleSkipIfContainsPRChanges(&result)
	}

	return result
}

func (d *DriftDetector) detectDrift(ctx context.Context, project models.TypedProject) (DriftProjectResult, error) {
	executor := d.newExecutor(project.Dir, project.Type)
	output, err := executor.Init(ctx, "-upgrade", "-lock=false", "-no-color")

	if err != nil {
		log.Info().Msgf("Error running init command in %s: %v", project.Dir, err)
		log.Info().Msg(output)
		return DriftProjectResult{Project: project, Drifted: false, Succeeded: false, InitOutput: output, PlanOutput: ""}, err
	}
	output, err = executor.Plan(ctx, "-lock=false", "-no-color")
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
