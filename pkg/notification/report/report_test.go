package report

import (
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"slices"
	"testing"
	"time"
)

func project(dir string, drifted, succeeded, skipped bool) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project:        models.TypedProject{Dir: dir, Type: models.Terraform},
		Drifted:        drifted,
		Succeeded:      succeeded,
		SkippedDueToPR: skipped,
	}
}

func TestClassifyBuckets(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			project("infra/drifted", true, true, false),
			project("infra/errored", false, false, false),
			project("infra/skipped", true, true, true),
			project("infra/clean", false, true, false),
		},
		TotalProjects: 4,
	}

	sum := Classify(result)

	if got := Dirs(sum.Drifted); !slices.Equal(got, []string{"infra/drifted"}) {
		t.Errorf("Drifted = %v", got)
	}
	if got := Dirs(sum.Errored); !slices.Equal(got, []string{"infra/errored"}) {
		t.Errorf("Errored = %v", got)
	}
	if got := Dirs(sum.Skipped); !slices.Equal(got, []string{"infra/skipped"}) {
		t.Errorf("Skipped = %v", got)
	}
	if got := Dirs(sum.Clean); !slices.Equal(got, []string{"infra/clean"}) {
		t.Errorf("Clean = %v", got)
	}
}

func TestClassifyErroredWinsOverDrifted(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{project("infra/a", true, false, false)},
		TotalProjects:  1,
	}

	sum := Classify(result)

	if sum.NumErrored() != 1 {
		t.Errorf("expected the project in Errored, got %d", sum.NumErrored())
	}
	if sum.NumDrifted() != 0 {
		t.Errorf("expected Drifted empty, got %d", sum.NumDrifted())
	}
}

func TestClassifySkippedRequiresDrifted(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{project("infra/a", false, true, true)},
		TotalProjects:  1,
	}

	sum := Classify(result)

	if sum.NumSkipped() != 0 {
		t.Errorf("expected Skipped empty for a non-drifted project, got %d", sum.NumSkipped())
	}
	if sum.NumClean() != 1 {
		t.Errorf("expected the project in Clean, got %d", sum.NumClean())
	}
}

func TestClassifyIgnoresStaleCounters(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			project("infra/a", true, true, false),
			project("infra/b", false, false, false),
		},
		TotalDrifted:  99,
		TotalErrored:  99,
		TotalSkipped:  99,
		TotalProjects: 2,
	}

	sum := Classify(result)

	if sum.NumDrifted() != 1 || sum.NumErrored() != 1 || sum.NumSkipped() != 0 {
		t.Errorf("counts must come from ProjectResults, got drifted=%d errored=%d skipped=%d",
			sum.NumDrifted(), sum.NumErrored(), sum.NumSkipped())
	}
}

func TestClassifyNotCheckedOnCancelledRun(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			project("infra/a", false, true, false),
			project("infra/b", false, true, false),
		},
		TotalProjects: 5,
	}

	sum := Classify(result)

	if sum.NotChecked != 3 {
		t.Errorf("NotChecked = %d, want 3", sum.NotChecked)
	}
	if sum.NumClean() != 2 {
		t.Errorf("NumClean = %d, want 2 (unreached projects are not clean)", sum.NumClean())
	}
}

func TestClassifyNotCheckedNeverNegative(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{project("infra/a", false, true, false)},
		TotalProjects:  0,
	}

	if got := Classify(result).NotChecked; got != 0 {
		t.Errorf("NotChecked = %d, want 0", got)
	}
}

func TestClassifySortsByDir(t *testing.T) {
	result := drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			project("infra/c", true, true, false),
			project("infra/a", true, true, false),
			project("infra/b", true, true, false),
		},
		TotalProjects: 3,
	}

	got := Dirs(Classify(result).Drifted)
	want := []string{"infra/a", "infra/b", "infra/c"}
	if !slices.Equal(got, want) {
		t.Errorf("Drifted = %v, want %v", got, want)
	}
}

func TestClassifyCarriesFailedPhase(t *testing.T) {
	failed := project("infra/a", false, false, false)
	failed.FailedPhase = drift.PhaseInit

	sum := Classify(drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{failed},
		TotalProjects:  1,
	})

	if sum.Errored[0].FailedPhase != drift.PhaseInit {
		t.Errorf("FailedPhase = %q, want %q", sum.Errored[0].FailedPhase, drift.PhaseInit)
	}
}

func TestClassifyCarriesTotals(t *testing.T) {
	sum := Classify(drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{project("infra/a", false, true, false)},
		TotalProjects:  7,
		Duration:       90 * time.Second,
	})

	if sum.TotalProjects != 7 {
		t.Errorf("TotalProjects = %d, want 7", sum.TotalProjects)
	}
	if sum.Duration != 90*time.Second {
		t.Errorf("Duration = %s, want 1m30s", sum.Duration)
	}
}

func TestHasFindings(t *testing.T) {
	tests := []struct {
		name    string
		results []drift.DriftProjectResult
		want    bool
	}{
		{"drift only", []drift.DriftProjectResult{project("a", true, true, false)}, true},
		{"error only", []drift.DriftProjectResult{project("a", false, false, false)}, true},
		{"skipped only", []drift.DriftProjectResult{project("a", true, true, true)}, false},
		{"clean only", []drift.DriftProjectResult{project("a", false, true, false)}, false},
		{"empty", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := Classify(drift.DriftDetectionResult{
				ProjectResults: tt.results,
				TotalProjects:  len(tt.results),
			})
			if got := sum.HasFindings(); got != tt.want {
				t.Errorf("HasFindings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirsOnEmptyBucket(t *testing.T) {
	if got := Dirs(nil); len(got) != 0 {
		t.Errorf("Dirs(nil) = %v, want empty", got)
	}
}

func TestDurationText(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"sub-second", 584274916 * time.Nanosecond, "584ms"},
		{"seconds", 30*time.Second + 412*time.Millisecond, "30.412s"},
		{"minutes rounds to seconds", 4*time.Minute + 12*time.Second + 341*time.Millisecond, "4m12s"},
		{"exactly one minute", time.Minute, "1m0s"},
		{"hours", 2*time.Hour + 3*time.Minute + 4400*time.Millisecond, "2h3m4s"},
		{"rounds half away from zero", 2*time.Hour + 3*time.Minute + 4500*time.Millisecond, "2h3m5s"},
		{"zero", 0, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Summary{Duration: tt.d}.DurationText()
			if got != tt.want {
				t.Errorf("DurationText() = %q, want %q", got, tt.want)
			}
		})
	}
}
