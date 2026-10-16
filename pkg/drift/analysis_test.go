package drift

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/exec"
	"driftive/pkg/models"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeExecutor stands in for terraform/tofu/terragrunt so DetectDrift can be exercised
// without those binaries installed. planOutput drives whether drift is reported: the real
// detector treats "Your infrastructure matches the configuration" as no drift.
type fakeExecutor struct {
	dir        string
	planOutput string

	// initOutput/initErr and planErr drive the failure paths. Zero values mean "succeeds".
	initOutput string
	initErr    error
	planErr    error

	// onInit, when set, runs at the start of Init outside mu, so tests can observe or hold a
	// project while it is running.
	onInit func(dir string)

	mu       *sync.Mutex
	initDirs *[]string
}

const noDriftOutput = "Your infrastructure matches the configuration"

func (f fakeExecutor) Dir() string { return f.dir }

func (f fakeExecutor) Init(_ context.Context, _ ...string) (string, error) {
	if f.onInit != nil {
		f.onInit(f.dir)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.initDirs = append(*f.initDirs, f.dir)
	if f.initErr != nil {
		return f.initOutput, f.initErr
	}
	return "init ok", nil
}

func (f fakeExecutor) Plan(_ context.Context, _ ...string) (string, error) {
	return f.planOutput, f.planErr
}

func (f fakeExecutor) ParsePlan(output string) string        { return output }
func (f fakeExecutor) ParseErrorOutput(output string) string { return output }

// newTestDetector wires a DriftDetector with a fake executor factory. initDirs records the
// working directory each executor was built with, so tests can assert that execution still
// uses the full discovered path.
func newTestDetector(repoDir string, projects []models.TypedProject, planOutput string) (*DriftDetector, *[]string) {
	var mu sync.Mutex
	initDirs := make([]string, 0)

	d := NewDriftDetector(
		repoDir,
		projects,
		&config.DriftiveConfig{Concurrency: 1, RepositoryPath: repoDir},
		&repo.DriftiveRepoConfig{},
		nil,
		nil,
	)
	d.newExecutor = func(dir string, _ models.ProjectType) exec.Executor {
		return fakeExecutor{dir: dir, planOutput: planOutput, mu: &mu, initDirs: &initDirs}
	}
	return &d, &initDirs
}

// newFailingTestDetector wires a detector whose executor fails at init or plan.
func newFailingTestDetector(projects []models.TypedProject, template fakeExecutor) *DriftDetector {
	var mu sync.Mutex
	initDirs := make([]string, 0)

	d := NewDriftDetector(
		".",
		projects,
		&config.DriftiveConfig{Concurrency: 1, RepositoryPath: "."},
		&repo.DriftiveRepoConfig{},
		nil,
		nil,
	)
	d.newExecutor = func(dir string, _ models.ProjectType) exec.Executor {
		e := template
		e.dir = dir
		e.mu = &mu
		e.initDirs = &initDirs
		return e
	}
	return &d
}

// newNonConcurrentTestDetector wires a detector with the given concurrency and non-concurrent
// patterns. onInit runs inside each project's Init.
func newNonConcurrentTestDetector(projects []models.TypedProject, concurrency int, patterns []string, onInit func(dir string)) *DriftDetector {
	var mu sync.Mutex
	initDirs := make([]string, 0)

	d := NewDriftDetector(
		".",
		projects,
		&config.DriftiveConfig{Concurrency: concurrency, RepositoryPath: "."},
		&repo.DriftiveRepoConfig{Settings: repo.DriftiveRepoConfigSettings{NonConcurrentPatterns: patterns}},
		nil,
		nil,
	)
	d.newExecutor = func(dir string, _ models.ProjectType) exec.Executor {
		return fakeExecutor{dir: dir, planOutput: noDriftOutput, onInit: onInit, mu: &mu, initDirs: &initDirs}
	}
	return &d
}

func resultDirs(result DriftDetectionResult) []string {
	dirs := make([]string, 0, len(result.ProjectResults))
	for _, r := range result.ProjectResults {
		dirs = append(dirs, r.Project.Dir)
	}
	return dirs
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// TestDetectDriftReportsRelativeDirs pins the invariant that makes normalizing the reported
// dir a no-op for GitHub Action runs: with --repo-path=./ the config layer trims to ".", the
// discovered dirs are already repo-relative, and the reported dir must come back byte-identical
// — including any literal dots, which the previous strings.ReplaceAll implementation stripped.
func TestDetectDriftReportsRelativeDirs(t *testing.T) {
	projects := []models.TypedProject{
		{Dir: "infra/foo.bar", Type: models.Terraform},
		{Dir: "infra/baz", Type: models.Terraform},
	}
	d, _ := newTestDetector(".", projects, noDriftOutput)

	result := d.DetectDrift(context.Background())

	dirs := resultDirs(result)
	if !contains(dirs, "infra/foo.bar") {
		t.Errorf("expected 'infra/foo.bar' to survive verbatim, got %v", dirs)
	}
	if !contains(dirs, "infra/baz") {
		t.Errorf("expected 'infra/baz', got %v", dirs)
	}
}

// TestDetectDriftNormalizesAbsoluteDirs covers the --repo-url / absolute --repo-path case,
// where the discovered dir carries a prefix that must not reach the API.
func TestDetectDriftNormalizesAbsoluteDirs(t *testing.T) {
	repoDir := "/var/folders/tmp/driftive123"
	projects := []models.TypedProject{
		{Dir: repoDir + "/infra/foo", Type: models.Terraform},
	}
	d, initDirs := newTestDetector(repoDir, projects, noDriftOutput)

	result := d.DetectDrift(context.Background())

	dirs := resultDirs(result)
	if len(dirs) != 1 || dirs[0] != "infra/foo" {
		t.Errorf("expected reported dir 'infra/foo', got %v", dirs)
	}

	// Execution must still happen in the real directory.
	if len(*initDirs) != 1 || (*initDirs)[0] != repoDir+"/infra/foo" {
		t.Errorf("expected the executor to run in %q, got %v", repoDir+"/infra/foo", *initDirs)
	}
}

// TestDetectDriftScansRepositoryRoot covers the project that used to be silently dropped:
// a repo whose Terraform lives at the root produced an empty relative dir and was skipped
// with no log line, leaving len(ProjectResults) < TotalProjects unexplained.
func TestDetectDriftScansRepositoryRoot(t *testing.T) {
	projects := []models.TypedProject{
		{Dir: ".", Type: models.Terraform},
	}
	d, _ := newTestDetector(".", projects, noDriftOutput)

	result := d.DetectDrift(context.Background())

	if len(result.ProjectResults) != 1 {
		t.Fatalf("expected the root project to be scanned, got %d result(s)", len(result.ProjectResults))
	}
	if result.ProjectResults[0].Project.Dir != "." {
		t.Errorf("expected the root project to report '.', got %q", result.ProjectResults[0].Project.Dir)
	}
	if result.TotalChecked != result.TotalProjects {
		t.Errorf("expected TotalChecked == TotalProjects for a complete scan, got %d vs %d",
			result.TotalChecked, result.TotalProjects)
	}
}

// TestDetectDriftTotalCheckedReflectsCancellation is the regression test for TotalChecked
// being hardcoded to len(d.Projects): a cancelled scan reported as fully checked.
func TestDetectDriftTotalCheckedReflectsCancellation(t *testing.T) {
	projects := []models.TypedProject{
		{Dir: "infra/a", Type: models.Terraform},
		{Dir: "infra/b", Type: models.Terraform},
		{Dir: "infra/c", Type: models.Terraform},
	}
	d, _ := newTestDetector(".", projects, noDriftOutput)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := d.DetectDrift(ctx)

	if result.TotalProjects != 3 {
		t.Errorf("expected TotalProjects to stay at the discovered count 3, got %d", result.TotalProjects)
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected TotalChecked 0 for a scan cancelled before any project ran, got %d", result.TotalChecked)
	}
}

// TestDetectDriftReportsProgress covers the live-reporting callbacks. The key assertion is that
// the dir handed to OnProjectStart matches the one on the finished result: the server upserts on
// (run, dir), so if the two disagreed — start reporting the discovered path and done the
// repo-relative one — nothing would deduplicate and every project would produce two rows.
func TestDetectDriftReportsProgress(t *testing.T) {
	repoDir := "/var/folders/tmp/driftive456"
	projects := []models.TypedProject{
		{Dir: repoDir + "/infra/a", Type: models.Terraform},
		{Dir: repoDir + "/infra/b", Type: models.Terraform},
	}
	d, _ := newTestDetector(repoDir, projects, noDriftOutput)

	var mu sync.Mutex
	started := make([]string, 0)
	finished := make([]string, 0)
	d.OnProjectStart = func(dir string) {
		mu.Lock()
		defer mu.Unlock()
		started = append(started, dir)
	}
	d.OnProjectDone = func(result DriftProjectResult) {
		mu.Lock()
		defer mu.Unlock()
		finished = append(finished, result.Project.Dir)
	}

	result := d.DetectDrift(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if len(started) != 2 {
		t.Errorf("expected OnProjectStart once per project, got %d call(s): %v", len(started), started)
	}
	if len(finished) != 2 {
		t.Errorf("expected OnProjectDone once per project, got %d call(s): %v", len(finished), finished)
	}

	// Both callbacks must agree, and both must be repo-relative.
	sort.Strings(started)
	sort.Strings(finished)
	want := []string{"infra/a", "infra/b"}
	if !slices.Equal(started, want) {
		t.Errorf("OnProjectStart dirs = %v, want %v", started, want)
	}
	if !slices.Equal(finished, started) {
		t.Errorf("callbacks disagree on the dir: start=%v done=%v", started, finished)
	}

	// And they must match what the terminal upload will send.
	reported := resultDirs(result)
	sort.Strings(reported)
	if !slices.Equal(reported, started) {
		t.Errorf("progress dirs %v disagree with the final result dirs %v", started, reported)
	}
}

// TestDetectDriftWithoutCallbacksIsUnaffected is the "API is optional" case: with both callbacks
// nil the scan must run exactly as before, with no panic.
func TestDetectDriftWithoutCallbacksIsUnaffected(t *testing.T) {
	projects := []models.TypedProject{
		{Dir: "infra/a", Type: models.Terraform},
	}
	d, _ := newTestDetector(".", projects, noDriftOutput)

	if d.OnProjectStart != nil || d.OnProjectDone != nil {
		t.Fatal("callbacks must default to nil so the scan path stays independent of the API")
	}

	result := d.DetectDrift(context.Background())
	if len(result.ProjectResults) != 1 {
		t.Errorf("expected 1 result with no callbacks wired, got %d", len(result.ProjectResults))
	}
}

// TestDetectDriftReportsDrift confirms the drift path still works through the fake executor
// and that counters line up with the per-project results.
func TestDetectDriftReportsDrift(t *testing.T) {
	projects := []models.TypedProject{
		{Dir: "infra/a", Type: models.Terraform},
		{Dir: "infra/b", Type: models.Terraform},
	}
	d, _ := newTestDetector(".", projects, "Plan: 1 to add, 0 to change, 0 to destroy.")

	result := d.DetectDrift(context.Background())

	if result.TotalDrifted != 2 {
		t.Errorf("expected 2 drifted projects, got %d", result.TotalDrifted)
	}
	if result.TotalErrored != 0 {
		t.Errorf("expected 0 errored projects, got %d", result.TotalErrored)
	}
	if result.TotalChecked != 2 {
		t.Errorf("expected TotalChecked 2, got %d", result.TotalChecked)
	}
}

func TestDetectDriftRecordsInitFailurePhase(t *testing.T) {
	projects := []models.TypedProject{{Dir: "infra/a", Type: models.Terraform}}
	d := newFailingTestDetector(projects, fakeExecutor{
		initOutput: "Error: Failed to install provider",
		initErr:    errors.New("exit status 1"),
	})

	result := d.DetectDrift(context.Background())

	got := result.ProjectResults[0]
	if got.Succeeded {
		t.Fatal("expected the project to be marked as failed")
	}
	if got.FailedPhase != PhaseInit {
		t.Errorf("FailedPhase = %q, want %q", got.FailedPhase, PhaseInit)
	}
	if got.InitOutput != "Error: Failed to install provider" {
		t.Errorf("InitOutput = %q, want the init output preserved", got.InitOutput)
	}
	if result.TotalErrored != 1 {
		t.Errorf("TotalErrored = %d, want 1", result.TotalErrored)
	}
}

func TestDetectDriftRecordsPlanFailurePhase(t *testing.T) {
	projects := []models.TypedProject{{Dir: "infra/a", Type: models.Terraform}}
	d := newFailingTestDetector(projects, fakeExecutor{
		planOutput: "Planning failed.",
		planErr:    errors.New("exit status 1"),
	})

	result := d.DetectDrift(context.Background())

	got := result.ProjectResults[0]
	if got.Succeeded {
		t.Fatal("expected the project to be marked as failed")
	}
	if got.FailedPhase != PhasePlan {
		t.Errorf("FailedPhase = %q, want %q", got.FailedPhase, PhasePlan)
	}
	if got.PlanOutput != "Planning failed." {
		t.Errorf("PlanOutput = %q, want the plan error output", got.PlanOutput)
	}
}

// TestDetectDriftInitFailureWithNoOutputKeepsErrorText covers a missing/unusable executable:
// CombinedOutput comes back empty, so without the fallback the issue body is an empty fence.
func TestDetectDriftInitFailureWithNoOutputKeepsErrorText(t *testing.T) {
	projects := []models.TypedProject{{Dir: "infra/a", Type: models.Terraform}}
	d := newFailingTestDetector(projects, fakeExecutor{
		initOutput: "",
		initErr:    errors.New("exec: \"terraform\": executable file not found in $PATH"),
	})

	result := d.DetectDrift(context.Background())

	got := result.ProjectResults[0]
	if got.FailedPhase != PhaseInit {
		t.Errorf("FailedPhase = %q, want %q", got.FailedPhase, PhaseInit)
	}
	if !strings.Contains(got.ErrorOutput(), "executable file not found") {
		t.Errorf("ErrorOutput() = %q, want it to carry the error text", got.ErrorOutput())
	}
}

func TestDetectDriftPlanFailureWithNoOutputKeepsErrorText(t *testing.T) {
	projects := []models.TypedProject{{Dir: "infra/a", Type: models.Terraform}}
	d := newFailingTestDetector(projects, fakeExecutor{
		planOutput: "",
		planErr:    errors.New("signal: killed"),
	})

	result := d.DetectDrift(context.Background())

	if got := result.ProjectResults[0].ErrorOutput(); !strings.Contains(got, "signal: killed") {
		t.Errorf("ErrorOutput() = %q, want it to carry the error text", got)
	}
}

func TestDetectDriftSuccessLeavesFailedPhaseEmpty(t *testing.T) {
	projects := []models.TypedProject{{Dir: "infra/a", Type: models.Terraform}}
	d, _ := newTestDetector(".", projects, noDriftOutput)

	result := d.DetectDrift(context.Background())

	if got := result.ProjectResults[0].FailedPhase; got != "" {
		t.Errorf("FailedPhase = %q, want empty for a successful project", got)
	}
}

func tfProjects(dirs ...string) []models.TypedProject {
	projects := make([]models.TypedProject, 0, len(dirs))
	for _, dir := range dirs {
		projects = append(projects, models.TypedProject{Dir: dir, Type: models.Terraform})
	}
	return projects
}

// TestDetectDriftSerializesNonConcurrentPatterns checks that projects matching the same pattern
// never overlap, even with free slots. Timing can only hide an overlap, never invent one.
func TestDetectDriftSerializesNonConcurrentPatterns(t *testing.T) {
	projects := tfProjects("gcp/confluent/a", "gcp/confluent/b", "gcp/confluent/c", "other/x", "other/y")

	var inFlight, maxInFlight atomic.Int32
	d := newNonConcurrentTestDetector(projects, 4, []string{"gcp/confluent/**"}, func(dir string) {
		if !strings.HasPrefix(dir, "gcp/confluent/") {
			return
		}
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
	})

	result := d.DetectDrift(context.Background())

	if got := maxInFlight.Load(); got > 1 {
		t.Errorf("expected at most 1 gcp/confluent project in flight, got %d", got)
	}
	if len(result.ProjectResults) != 5 || result.TotalChecked != 5 {
		t.Errorf("expected all 5 projects checked, got %d result(s), TotalChecked %d",
			len(result.ProjectResults), result.TotalChecked)
	}
}

// TestDetectDriftNonConcurrentDoesNotBlockOthers holds one gcp/confluent project open and checks
// that the other confluent project waits while unrelated projects still run. Concurrency 2 makes
// it catch a waiting project that holds a slot: it would leave no slot for other/*.
func TestDetectDriftNonConcurrentDoesNotBlockOthers(t *testing.T) {
	projects := tfProjects("gcp/confluent/a", "gcp/confluent/b", "other/x", "other/y")

	unblock := make(chan struct{})
	firstConfluentStarted := make(chan struct{})
	var confluentStarted atomic.Int32
	var othersStarted sync.WaitGroup
	othersStarted.Add(2)

	d := newNonConcurrentTestDetector(projects, 2, []string{"gcp/confluent/**"}, func(dir string) {
		if !strings.HasPrefix(dir, "gcp/confluent/") {
			othersStarted.Done()
			return
		}
		if confluentStarted.Add(1) == 1 {
			close(firstConfluentStarted)
			<-unblock
		}
	})

	done := make(chan DriftDetectionResult, 1)
	go func() { done <- d.DetectDrift(context.Background()) }()

	othersDone := make(chan struct{})
	go func() {
		othersStarted.Wait()
		<-firstConfluentStarted
		close(othersDone)
	}()

	select {
	case <-othersDone:
	case <-time.After(5 * time.Second):
		close(unblock)
		t.Fatal("other/* projects did not run while a gcp/confluent project was in flight")
	}

	if got := confluentStarted.Load(); got != 1 {
		t.Errorf("expected the second gcp/confluent project to wait, but %d started", got)
	}

	close(unblock)
	result := <-done
	if len(result.ProjectResults) != 4 {
		t.Errorf("expected all 4 projects checked, got %d", len(result.ProjectResults))
	}
}

// TestDetectDriftNonConcurrentCancelledWhileWaiting cancels the run from inside the first project:
// the project waiting on its group must not be started afterwards.
func TestDetectDriftNonConcurrentCancelledWhileWaiting(t *testing.T) {
	projects := tfProjects("gcp/confluent/a", "gcp/confluent/b")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newNonConcurrentTestDetector(projects, 4, []string{"gcp/confluent/**"}, func(string) {
		cancel()
	})

	result := d.DetectDrift(ctx)

	if result.TotalChecked != 1 || len(result.ProjectResults) != 1 {
		t.Errorf("expected only the first project checked, got TotalChecked %d and %d result(s)",
			result.TotalChecked, len(result.ProjectResults))
	}
}
