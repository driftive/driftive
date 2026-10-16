package drift

import (
	"context"
	"driftive/pkg/models"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (d *DriftDetector) detectDriftConcurrently(ctx context.Context, project models.TypedProject, projectDir string, groups []int, released chan<- []int) {
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

	// Before the deferred semaphore release, so by the time the dispatcher can take this slot it
	// can also see the project's non-concurrent groups as free.
	released <- groups
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

	projectDirs := make([]string, len(d.Projects))
	for i, proj := range d.Projects {
		projectDirs[i] = relativeProjectDir(d.RepoDir, proj.Dir)
	}

	patterns := d.RepoConfig.Settings.NonConcurrentPatterns
	groups, err := nonConcurrentGroups(patterns, projectDirs)
	if err != nil {
		log.Error().Msgf("Invalid non-concurrent patterns, ignoring them: %v", err)
		patterns = nil
		groups = make([][]int, len(d.Projects))
	}
	if len(patterns) > 0 {
		matched := 0
		for _, g := range groups {
			if len(g) > 0 {
				matched++
			}
		}
		log.Info().Msgf("%d project(s) matched non-concurrent patterns", matched)
	}

	// busy is owned by this goroutine; workers hand their groups back through released, which is
	// buffered for every project so a worker never blocks on it.
	busy := make([]bool, len(patterns))
	released := make(chan []int, len(d.Projects))
	pending := make([]int, len(d.Projects))
	for i := range pending {
		pending[i] = i
	}

	for len(pending) > 0 {
		// Honor cancellation between projects so an interrupted run doesn't keep
		// spinning up new terraform/tofu processes after Ctrl-C.
		if ctx.Err() != nil {
			log.Info().Msgf("Drift analysis cancelled after %d/%d projects: %v", totalChecked, len(d.Projects), ctx.Err())
			break
		}

		d.semaphore <- struct{}{}
		pos := waitForRunnable(ctx, pending, groups, busy, released)
		if pos < 0 || ctx.Err() != nil {
			// Cancelled while waiting for a slot or a non-concurrent group; the check above reports it.
			<-d.semaphore
			continue
		}

		idx := pending[pos]
		pending = slices.Delete(pending, pos, pos+1)
		setBusy(groups[idx], busy, true)

		proj := d.Projects[idx]
		totalChecked++
		log.Info().Msgf("Checking drift in project %d/%d: %s (%s)", totalChecked, len(d.Projects), projectDirs[idx], models.ProjectTypeToStr(proj.Type))
		d.workerWg.Add(1)
		go d.detectDriftConcurrently(ctx, proj, projectDirs[idx], groups[idx], released)
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
		return DriftProjectResult{Project: project, Drifted: false, Succeeded: false,
			FailedPhase: PhaseInit, InitOutput: orErrorText(output, err), PlanOutput: ""}, err
	}
	output, err = executor.Plan(ctx, "-lock=false", "-no-color")
	if err != nil {
		log.Info().Msgf("Error running plan command in %s: %v", project.Dir, err)
		log.Info().Msg(output)
		return DriftProjectResult{Project: project, Drifted: false, Succeeded: false,
			FailedPhase: PhasePlan, InitOutput: "", PlanOutput: orErrorText(executor.ParseErrorOutput(output), err)}, err
	}
	driftDetected := d.isDriftDetected(output)
	if driftDetected {
		output = executor.ParsePlan(output)
	}
	result := DriftProjectResult{Project: project, Drifted: driftDetected, Succeeded: true, InitOutput: "", PlanOutput: output}
	return result, nil
}

// orErrorText falls back to the error text when the command produced no output, which happens
// when the executable itself could not be run.
func orErrorText(output string, err error) string {
	if strings.TrimSpace(output) == "" && err != nil {
		return err.Error()
	}
	return output
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
