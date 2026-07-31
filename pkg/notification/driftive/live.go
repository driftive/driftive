package driftive

import (
	"context"
	"driftive/pkg/drift"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"resty.dev/v3"
)

const (
	// defaultFlushInterval is how often buffered progress is posted.
	defaultFlushInterval = 3 * time.Second
	// defaultHeartbeatInterval bounds how long the API can go without hearing from a live run,
	// even when nothing changed. It keeps the run's updated_at fresh so a long single-project plan
	// stays distinguishable from a dead runner (the server sweeps runs stale for 15 minutes).
	defaultHeartbeatInterval = 15 * time.Second
	// requestTimeout keeps a hung request from stalling the flusher indefinitely.
	requestTimeout = 5 * time.Second
)

// progressRequest is the body of POST /api/v1/drift_analysis/progress.
type progressRequest struct {
	TotalProjects  int                        `json:"total_projects"`
	Running        []string                   `json:"running"`
	ProjectResults []drift.DriftProjectResult `json:"project_results"`
}

// LiveReporter posts partial results to the Driftive API while a scan is still running, so the
// dashboard fills in as projects finish instead of appearing all at once at the end.
//
// It is strictly best-effort: nothing it does can fail, slow or change the outcome of the scan.
// ProjectStarted/ProjectFinished only take a mutex and mutate in-memory state; all I/O happens on
// a single flusher goroutine. A failed post keeps its buffer and re-sends on the next tick, which
// is safe because the server upserts by (run, dir).
type LiveReporter struct {
	url           string
	token         string
	runKey        string
	totalProjects int

	client *resty.Client

	// Tick intervals, defaulted by NewLiveReporter. Tests shorten them so the suite does not
	// sleep on real 3s ticks; nothing outside this package sets them.
	flushInterval     time.Duration
	heartbeatInterval time.Duration

	mu      sync.Mutex
	running map[string]struct{}
	pending []drift.DriftProjectResult
	// version increments on every state change; lastSentVersion is the version the last accepted
	// post carried. Comparing the two — rather than clearing a dirty flag — is what keeps a change
	// that lands while a request is in flight from being swallowed.
	version         uint64
	lastSentVersion uint64

	// urlLogged keeps the dashboard link to a single log line across ticks.
	urlLogged atomic.Bool

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

func NewLiveReporter(url, token, runKey string, totalProjects int) *LiveReporter {
	return &LiveReporter{
		url:           url,
		token:         token,
		runKey:        runKey,
		totalProjects: totalProjects,
		// No retry configuration: the buffer-and-resend-on-next-tick loop is the retry, and it is
		// bounded by the scan's own duration. Unlike the terminal upload, which gets one shot.
		client:            resty.New().SetTimeout(requestTimeout),
		flushInterval:     defaultFlushInterval,
		heartbeatInterval: defaultHeartbeatInterval,
		running:           make(map[string]struct{}),
		pending:           make([]drift.DriftProjectResult, 0),
		// version starts ahead of lastSentVersion so the run is created on the first tick even if
		// no project has finished yet, which is what yields the dashboard URL early.
		version: 1,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// ProjectStarted records that a project's analysis has begun. Safe for concurrent use.
func (r *LiveReporter) ProjectStarted(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[dir] = struct{}{}
	r.version++
}

// ProjectFinished records a finished project. Safe for concurrent use.
func (r *LiveReporter) ProjectFinished(result drift.DriftProjectResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, result.Project.Dir)
	r.pending = append(r.pending, result)
	r.version++
}

// Start launches the single flusher goroutine.
func (r *LiveReporter) Start(ctx context.Context) {
	go r.flushLoop(ctx)
}

// Stop shuts the flusher down and blocks until it has exited, bounded by requestTimeout if a
// request is in flight. It must be called before the terminal upload, so a progress post cannot
// race the finalize.
func (r *LiveReporter) Stop() {
	r.stopOnce.Do(func() {
		close(r.stop)
		<-r.done
		r.client.Close()
	})
}

func (r *LiveReporter) flushLoop(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()

	// Flush once up front rather than waiting a full interval. This creates the run immediately so
	// the dashboard URL is available before any plan finishes, and it means a scan shorter than one
	// flush interval still reports something.
	lastPost := r.flushOnce(ctx, time.Time{})

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stop:
			return
		case <-ticker.C:
			lastPost = r.flushOnce(ctx, lastPost)
		}
	}
}

// flushOnce posts the current snapshot if there is anything to say, returning the time of the last
// accepted post (unchanged when nothing was sent or the request failed).
func (r *LiveReporter) flushOnce(ctx context.Context, lastPost time.Time) time.Time {
	heartbeatDue := lastPost.IsZero() || time.Since(lastPost) >= r.heartbeatInterval
	results, running, version, ok := r.takeSnapshot(heartbeatDue)
	if !ok {
		return lastPost
	}
	if !r.post(ctx, running, results) {
		return lastPost
	}
	r.commit(version, len(results))
	return time.Now()
}

// takeSnapshot returns the buffered results, the current running set, and the version they
// represent. ok is false when nothing changed and no heartbeat is due, so the tick issues no
// request at all.
func (r *LiveReporter) takeSnapshot(heartbeatDue bool) (results []drift.DriftProjectResult, running []string, version uint64, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.version == r.lastSentVersion && !heartbeatDue {
		return nil, nil, 0, false
	}

	results = make([]drift.DriftProjectResult, len(r.pending))
	copy(results, r.pending)

	running = make([]string, 0, len(r.running))
	for dir := range r.running {
		running = append(running, dir)
	}
	return results, running, r.version, true
}

// commit marks the snapshot as delivered and drops the results that were actually sent. Anything
// buffered while the request was in flight keeps a higher version, so the next tick still fires.
func (r *LiveReporter) commit(version uint64, sent int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sent >= len(r.pending) {
		r.pending = r.pending[:0]
	} else {
		r.pending = append(r.pending[:0], r.pending[sent:]...)
	}
	r.lastSentVersion = version
}

// post sends one progress request and reports whether it was accepted. Failures are logged at
// debug and never propagate: the buffer is kept and re-sent on the next tick.
func (r *LiveReporter) post(ctx context.Context, running []string, results []drift.DriftProjectResult) bool {
	body := progressRequest{
		TotalProjects:  r.totalProjects,
		Running:        running,
		ProjectResults: results,
	}

	res, err := r.client.R().
		WithContext(ctx).
		SetHeader("X-Token", r.token).
		SetHeader("Idempotency-Key", r.runKey).
		SetBody(body).
		Post(r.url + "/api/v1/drift_analysis/progress")

	if err != nil {
		log.Debug().Msgf("Live progress report failed, will retry on the next tick: %v", err)
		return false
	}
	if res.StatusCode() != 200 {
		log.Debug().Msgf("Live progress report returned status %d, will retry on the next tick", res.StatusCode())
		return false
	}

	r.logDashboardURLOnce(res.Bytes())
	return true
}

// logDashboardURLOnce surfaces the dashboard link on the first accepted post, so it is available
// while the scan is still running rather than only after the terminal upload.
func (r *LiveReporter) logDashboardURLOnce(payload []byte) {
	if r.urlLogged.Swap(true) {
		return
	}
	var response AnalysisResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		log.Debug().Msgf("Could not parse the live progress response: %v", err)
		return
	}
	if response.DashboardURL != "" {
		log.Info().Msgf("Dashboard URL: %s", response.DashboardURL)
	}
}
