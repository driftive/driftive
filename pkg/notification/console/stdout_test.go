package console

import (
	"bytes"
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func handleAndCapture(t *testing.T, result drift.DriftDetectionResult) string {
	t.Helper()
	var buf bytes.Buffer
	s := Stdout{logger: zerolog.New(&buf)}
	if err := s.Handle(context.Background(), result); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	return buf.String()
}

func result(results []drift.DriftProjectResult, total int) drift.DriftDetectionResult {
	return drift.DriftDetectionResult{
		ProjectResults: results,
		TotalProjects:  total,
		Duration:       4*time.Minute + 12*time.Second,
	}
}

func projectResult(dir string, drifted, succeeded, skipped bool, phase string) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project:        models.TypedProject{Dir: dir},
		Drifted:        drifted,
		Succeeded:      succeeded,
		SkippedDueToPR: skipped,
		FailedPhase:    phase,
	}
}

// TestStdoutPrintsOnCleanRun covers the early return that made an all-clean or all-errors run
// log nothing at all.
func TestStdoutPrintsOnCleanRun(t *testing.T) {
	out := handleAndCapture(t, result([]drift.DriftProjectResult{
		projectResult("infra/a", false, true, false, ""),
	}, 1))

	if !strings.Contains(out, "No drift or errors detected") {
		t.Errorf("expected an all-clear line, got:\n%s", out)
	}
	if !strings.Contains(out, "1 projects: 0 drifted, 0 errored, 0 skipped, 1 clean") {
		t.Errorf("expected the counts line, got:\n%s", out)
	}
}

func TestStdoutReportsErroredProjects(t *testing.T) {
	out := handleAndCapture(t, result([]drift.DriftProjectResult{
		projectResult("infra/prod/iam", false, false, false, drift.PhasePlan),
		projectResult("modules/net", false, false, false, drift.PhaseInit),
	}, 2))

	if !strings.Contains(out, "Projects that failed to analyze") {
		t.Errorf("expected an errored section, got:\n%s", out)
	}
	if !strings.Contains(out, "infra/prod/iam") || !strings.Contains(out, "modules/net") {
		t.Errorf("expected both errored projects listed, got:\n%s", out)
	}
	if !strings.Contains(out, "0 drifted, 2 errored") {
		t.Errorf("expected the errored count, got:\n%s", out)
	}
}

func TestStdoutShowsFailedPhase(t *testing.T) {
	out := handleAndCapture(t, result([]drift.DriftProjectResult{
		projectResult("infra/prod/iam", false, false, false, drift.PhasePlan),
		projectResult("modules/net", false, false, false, drift.PhaseInit),
	}, 2))

	if !strings.Contains(out, "infra/prod/iam (plan)") {
		t.Errorf("expected the plan phase, got:\n%s", out)
	}
	if !strings.Contains(out, "modules/net (init)") {
		t.Errorf("expected the init phase, got:\n%s", out)
	}
}

func TestStdoutListsSkippedProjects(t *testing.T) {
	out := handleAndCapture(t, result([]drift.DriftProjectResult{
		projectResult("infra/prod/vpc", true, true, false, ""),
		projectResult("infra/dev/vpc", true, true, true, ""),
	}, 2))

	if !strings.Contains(out, "Skipped due to open PRs") {
		t.Errorf("expected a skipped section, got:\n%s", out)
	}
	if !strings.Contains(out, "infra/dev/vpc") {
		t.Errorf("expected the skipped project listed, got:\n%s", out)
	}
	if !strings.Contains(out, "Projects with state drift") {
		t.Errorf("expected a drift section, got:\n%s", out)
	}
}

func TestStdoutOmitsEmptySections(t *testing.T) {
	out := handleAndCapture(t, result([]drift.DriftProjectResult{
		projectResult("infra/prod/vpc", true, true, false, ""),
	}, 1))

	if strings.Contains(out, "failed to analyze") {
		t.Errorf("expected no errored section, got:\n%s", out)
	}
	if strings.Contains(out, "Skipped due to open PRs") {
		t.Errorf("expected no skipped section, got:\n%s", out)
	}
	if strings.Contains(out, "No drift or errors detected") {
		t.Errorf("expected no all-clear line when drift was found, got:\n%s", out)
	}
}

func TestNewStdoutUsesGlobalLogger(t *testing.T) {
	if err := (NewStdout()).Handle(context.Background(), result(nil, 0)); err != nil {
		t.Errorf("Handle() error = %v", err)
	}
}
