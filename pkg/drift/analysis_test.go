package drift

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/exec"
	"driftive/pkg/models"
	"slices"
	"sort"
	"sync"
	"testing"
)

// fakeExecutor stands in for terraform/tofu/terragrunt so DetectDrift can be exercised
// without those binaries installed. planOutput drives whether drift is reported: the real
// detector treats "Your infrastructure matches the configuration" as no drift.
type fakeExecutor struct {
	dir        string
	planOutput string

	mu       *sync.Mutex
	initDirs *[]string
}

const noDriftOutput = "Your infrastructure matches the configuration"

func (f fakeExecutor) Dir() string { return f.dir }

func (f fakeExecutor) Init(_ context.Context, _ ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.initDirs = append(*f.initDirs, f.dir)
	return "init ok", nil
}

func (f fakeExecutor) Plan(_ context.Context, _ ...string) (string, error) {
	return f.planOutput, nil
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
