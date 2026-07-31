package driftive

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// capturedRequest is one progress post as the fake API saw it.
type capturedRequest struct {
	token  string
	key    string
	body   progressRequest
	rawURL string
}

// progressRecorder is a fake API that records every progress post and can be told to fail.
type progressRecorder struct {
	mu       sync.Mutex
	requests []capturedRequest
	// statusFor returns the status to reply with for the Nth request (1-indexed).
	statusFor func(n int) int
}

func (p *progressRecorder) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var body progressRequest
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("unmarshal body %s: %v", raw, err)
		}

		p.mu.Lock()
		p.requests = append(p.requests, capturedRequest{
			token:  r.Header.Get("X-Token"),
			key:    r.Header.Get("Idempotency-Key"),
			body:   body,
			rawURL: r.URL.Path,
		})
		n := len(p.requests)
		p.mu.Unlock()

		status := http.StatusOK
		if p.statusFor != nil {
			status = p.statusFor(n)
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"run_id":"abc","dashboard_url":"http://dash/run/abc"}`))
		}
	}
}

func (p *progressRecorder) snapshot() []capturedRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]capturedRequest, len(p.requests))
	copy(out, p.requests)
	return out
}

func (p *progressRecorder) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

// waitForRequests blocks until the recorder has seen at least n requests, or fails the test.
func (p *progressRecorder) waitForRequests(t *testing.T, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if p.count() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d progress request(s), got %d", n, p.count())
}

// testFlushInterval keeps the suite fast; the production defaults are 3s / 15s.
const (
	testFlushInterval     = 20 * time.Millisecond
	testHeartbeatInterval = 10 * time.Second
)

const testToken = "secret-token"

// newTestReporter builds a reporter that ticks fast enough for a unit test. The heartbeat is left
// far outside the test window so only real changes trigger a post.
func newTestReporter(url, runKey string, totalProjects int) *LiveReporter {
	r := NewLiveReporter(url, testToken, runKey, totalProjects)
	r.flushInterval = testFlushInterval
	r.heartbeatInterval = testHeartbeatInterval
	return r
}

func finishedProject(dir string, drifted bool) drift.DriftProjectResult {
	return drift.DriftProjectResult{
		Project:    models.TypedProject{Dir: dir, Type: models.Terraform},
		Drifted:    drifted,
		Succeeded:  true,
		InitOutput: "init",
		PlanOutput: "plan for " + dir,
	}
}

// TestLiveReporter_FlushSendsHeadersAndRelativeDirs pins the wire format: the auth header, the run
// key that ties progress to the finalize, and repo-relative dirs in both `running` and
// `project_results` (the API upserts on (run, dir), so these must agree).
func TestLiveReporter_FlushSendsHeadersAndRelativeDirs(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := newTestReporter(server.URL, "run-key-1", 3)
	r.Start(context.Background())
	defer r.Stop()

	r.ProjectStarted("infra/a")
	r.ProjectStarted("infra/b")
	r.ProjectFinished(finishedProject("infra/a", true))

	rec.waitForRequests(t, 1, 5*time.Second)
	got := rec.snapshot()[0]

	if got.rawURL != "/api/v1/drift_analysis/progress" {
		t.Errorf("expected path /api/v1/drift_analysis/progress, got %s", got.rawURL)
	}
	if got.token != "secret-token" {
		t.Errorf("expected X-Token 'secret-token', got %q", got.token)
	}
	if got.key != "run-key-1" {
		t.Errorf("expected Idempotency-Key 'run-key-1', got %q", got.key)
	}
	if got.body.TotalProjects != 3 {
		t.Errorf("expected total_projects 3, got %d", got.body.TotalProjects)
	}

	// a finished, so only b is still running.
	if len(got.body.Running) != 1 || got.body.Running[0] != "infra/b" {
		t.Errorf("expected running ['infra/b'], got %v", got.body.Running)
	}
	if len(got.body.ProjectResults) != 1 {
		t.Fatalf("expected 1 project result, got %d", len(got.body.ProjectResults))
	}
	if dir := got.body.ProjectResults[0].Project.Dir; dir != "infra/a" {
		t.Errorf("expected repo-relative dir 'infra/a', got %q", dir)
	}
	if !got.body.ProjectResults[0].Drifted {
		t.Error("expected the drifted flag to survive the round trip")
	}
}

// TestLiveReporter_PostsImmediatelyWithoutWaitingForATick is the regression test for a scan that
// finishes inside one flush interval reporting nothing at all: with time.NewTicker the first tick
// only fires after a full interval, so a 2.5s scan created no run and never surfaced a dashboard
// URL. The run must be created promptly instead, well before the first tick.
func TestLiveReporter_PostsImmediatelyWithoutWaitingForATick(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := NewLiveReporter(server.URL, testToken, "run-key-immediate", 3)
	// Long enough that a tick cannot plausibly fire during this test.
	r.flushInterval = 30 * time.Second
	r.heartbeatInterval = 30 * time.Second
	r.Start(context.Background())
	defer r.Stop()

	rec.waitForRequests(t, 1, 5*time.Second)

	got := rec.snapshot()[0]
	if got.body.TotalProjects != 3 {
		t.Errorf("expected the opening post to announce total_projects 3, got %d", got.body.TotalProjects)
	}
	if len(got.body.ProjectResults) != 0 {
		t.Errorf("expected the opening post to carry no results yet, got %d", len(got.body.ProjectResults))
	}
}

// TestLiveReporter_ShortScanStillReportsProgress is the end-to-end shape of the same bug: a scan
// that starts, finishes one project and stops faster than a flush interval must still have told
// the API about it.
func TestLiveReporter_ShortScanStillReportsProgress(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := NewLiveReporter(server.URL, testToken, "run-key-short", 1)
	r.flushInterval = 30 * time.Second
	r.heartbeatInterval = 30 * time.Second
	r.Start(context.Background())

	r.ProjectStarted("infra/a")
	r.ProjectFinished(finishedProject("infra/a", true))
	rec.waitForRequests(t, 1, 5*time.Second)
	r.Stop()

	if n := rec.count(); n == 0 {
		t.Fatal("a scan shorter than one flush interval reported nothing to the API")
	}
}

// TestLiveReporter_NoChangeIssuesNoRequest verifies ticks are skipped when nothing changed, so an
// idle scan does not hammer the API. The heartbeat is set well outside this window.
func TestLiveReporter_NoChangeIssuesNoRequest(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := newTestReporter(server.URL, "run-key-2", 1)
	r.Start(context.Background())
	defer r.Stop()

	// The first tick always fires, to create the run and yield the dashboard URL early.
	rec.waitForRequests(t, 1, 5*time.Second)

	// Nothing changes from here; subsequent ticks must be skipped.
	time.Sleep(10 * testFlushInterval)

	if n := rec.count(); n != 1 {
		t.Errorf("expected no further requests while idle inside the heartbeat window, got %d total", n)
	}
}

// TestLiveReporter_FailedFlushRetainsAndResends is the reliability property: a 500 must neither
// panic nor block the next tick, and the buffered results must be re-sent rather than lost.
func TestLiveReporter_FailedFlushRetainsAndResends(t *testing.T) {
	rec := &progressRecorder{
		statusFor: func(n int) int {
			if n == 1 {
				return http.StatusInternalServerError
			}
			return http.StatusOK
		},
	}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := newTestReporter(server.URL, "run-key-3", 2)
	r.Start(context.Background())
	defer r.Stop()

	r.ProjectFinished(finishedProject("infra/a", true))

	rec.waitForRequests(t, 2, 10*time.Second)
	got := rec.snapshot()

	// The rejected result must appear again on the retry.
	if len(got[0].body.ProjectResults) != 1 {
		t.Fatalf("first request carried %d result(s), want 1", len(got[0].body.ProjectResults))
	}
	if len(got[1].body.ProjectResults) != 1 {
		t.Fatalf("the 500 dropped the buffered result: retry carried %d result(s), want 1",
			len(got[1].body.ProjectResults))
	}
	if got[1].body.ProjectResults[0].Project.Dir != "infra/a" {
		t.Errorf("retry re-sent %q, want 'infra/a'", got[1].body.ProjectResults[0].Project.Dir)
	}
}

// TestLiveReporter_AcceptedResultsAreNotResent is the other half: once a flush succeeds its
// results are dropped, so a long scan does not re-upload every plan output on every tick.
func TestLiveReporter_AcceptedResultsAreNotResent(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := newTestReporter(server.URL, "run-key-4", 2)
	r.Start(context.Background())
	defer r.Stop()

	r.ProjectFinished(finishedProject("infra/a", true))
	rec.waitForRequests(t, 1, 5*time.Second)

	// A second project finishing must trigger a tick carrying only that project.
	r.ProjectFinished(finishedProject("infra/b", false))
	rec.waitForRequests(t, 2, 5*time.Second)

	got := rec.snapshot()
	if len(got[1].body.ProjectResults) != 1 {
		t.Fatalf("second flush carried %d result(s), want only the newly finished one",
			len(got[1].body.ProjectResults))
	}
	if got[1].body.ProjectResults[0].Project.Dir != "infra/b" {
		t.Errorf("second flush re-sent %q instead of only 'infra/b'",
			got[1].body.ProjectResults[0].Project.Dir)
	}
}

// TestLiveReporter_StopIsSafeWithUnreachableAPI covers the "API is down" path end to end: the
// reporter must not panic, must not block Stop indefinitely, and must swallow every error.
func TestLiveReporter_StopIsSafeWithUnreachableAPI(t *testing.T) {
	r := newTestReporter("http://127.0.0.1:59998", "run-key-5", 1)
	r.Start(context.Background())

	r.ProjectStarted("infra/a")
	r.ProjectFinished(finishedProject("infra/a", true))
	time.Sleep(10 * testFlushInterval)

	done := make(chan struct{})
	go func() {
		r.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop blocked with an unreachable API")
	}

	// Stop must be idempotent — a second call must not panic on a closed channel.
	r.Stop()
}

// TestLiveReporter_StopEndsTheFlusherBeforeFinalize pins the ordering guarantee: after Stop
// returns, no further progress post can be issued to race the terminal upload.
func TestLiveReporter_StopEndsTheFlusherBeforeFinalize(t *testing.T) {
	rec := &progressRecorder{}
	server := httptest.NewServer(rec.handler(t))
	defer server.Close()

	r := newTestReporter(server.URL, "run-key-6", 1)
	r.Start(context.Background())

	r.ProjectFinished(finishedProject("infra/a", true))
	rec.waitForRequests(t, 1, 5*time.Second)

	r.Stop()
	countAtStop := rec.count()

	// Anything recorded after Stop would be a post racing the finalize.
	r.ProjectFinished(finishedProject("infra/b", true))
	time.Sleep(10 * testFlushInterval)

	if n := rec.count(); n != countAtStop {
		t.Errorf("flusher issued %d request(s) after Stop returned", n-countAtStop)
	}
}
