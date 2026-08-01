// Package report classifies a drift run into the buckets every notifier renders from.
package report

import (
	"driftive/pkg/drift"
	"sort"
	"time"
)

// Status is the mutually exclusive bucket a project's result falls into.
type Status string

const (
	StatusDrifted Status = "drifted"
	StatusErrored Status = "errored"
	StatusSkipped Status = "skipped"
	StatusClean   Status = "clean"
)

// Project is one project's outcome, bucketed and ready to render.
type Project struct {
	Dir    string
	Status Status
	// FailedPhase is drift.PhaseInit or drift.PhasePlan when Status is StatusErrored.
	FailedPhase string
}

// Summary is a whole run's classification.
//
// Counts are derived from ProjectResults rather than DriftDetectionResult's Total* fields:
// TotalDrifted and TotalSkipped are maintained by mutation and only when skip_if_open_pr is
// enabled, there is no clean counter, and TotalProjects counts discovered projects, which
// exceeds len(ProjectResults) when a run is cancelled.
type Summary struct {
	Drifted []Project
	Errored []Project
	Skipped []Project
	Clean   []Project

	// TotalProjects is how many projects were discovered.
	TotalProjects int
	// NotChecked is how many discovered projects the run never reached. Zero for a complete run.
	NotChecked int
	Duration   time.Duration
}

// Classify buckets a run's results. Each bucket is sorted by Dir so message bodies stay stable
// across runs — ProjectResults arrives in goroutine-completion order.
func Classify(result drift.DriftDetectionResult) Summary {
	sum := Summary{
		TotalProjects: result.TotalProjects,
		Duration:      result.Duration,
	}

	for _, r := range result.ProjectResults {
		p := Project{Dir: r.Project.Dir}
		switch {
		case !r.Succeeded:
			p.Status = StatusErrored
			p.FailedPhase = r.FailedPhase
			sum.Errored = append(sum.Errored, p)
		case r.Drifted && r.SkippedDueToPR:
			p.Status = StatusSkipped
			sum.Skipped = append(sum.Skipped, p)
		case r.Drifted:
			p.Status = StatusDrifted
			sum.Drifted = append(sum.Drifted, p)
		default:
			p.Status = StatusClean
			sum.Clean = append(sum.Clean, p)
		}
	}

	if n := result.TotalProjects - len(result.ProjectResults); n > 0 {
		sum.NotChecked = n
	}

	for _, bucket := range [][]Project{sum.Drifted, sum.Errored, sum.Skipped, sum.Clean} {
		sortByDir(bucket)
	}

	return sum
}

func sortByDir(projects []Project) {
	sort.Slice(projects, func(i, j int) bool { return projects[i].Dir < projects[j].Dir })
}

func (s Summary) NumDrifted() int { return len(s.Drifted) }
func (s Summary) NumErrored() int { return len(s.Errored) }
func (s Summary) NumSkipped() int { return len(s.Skipped) }
func (s Summary) NumClean() int   { return len(s.Clean) }

// HasFindings reports whether the run produced anything worth notifying about. Skipped-only and
// fully clean runs are not findings.
func (s Summary) HasFindings() bool {
	return len(s.Drifted) > 0 || len(s.Errored) > 0
}

// DurationText formats the run duration for display, trading precision for readability once
// the run is long enough that sub-second digits are noise.
func (s Summary) DurationText() string {
	if s.Duration >= time.Minute {
		return s.Duration.Round(time.Second).String()
	}
	return s.Duration.Round(time.Millisecond).String()
}

// Dirs returns the dirs of the given projects, in bucket order.
func Dirs(projects []Project) []string {
	dirs := make([]string, 0, len(projects))
	for _, p := range projects {
		dirs = append(dirs, p.Dir)
	}
	return dirs
}
