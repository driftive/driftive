package driftive

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/models"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func sampleResult() drift.DriftDetectionResult {
	return drift.DriftDetectionResult{
		ProjectResults: []drift.DriftProjectResult{
			{
				Project:    models.TypedProject{Dir: "infra/foo.bar", Type: models.Terraform},
				Drifted:    true,
				Succeeded:  true,
				InitOutput: "init",
				PlanOutput: "plan",
			},
		},
		TotalDrifted:  1,
		TotalProjects: 1,
		TotalChecked:  1,
		Duration:      time.Minute,
	}
}

func TestHandle_SendsPayloadAndHeaders(t *testing.T) {
	var (
		gotPath  string
		gotToken string
		gotKey   string
		gotBody  []byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Token")
		gotKey = r.Header.Get("Idempotency-Key")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"run_id":"abc","dashboard_url":"http://dash/run/abc"}`))
	}))
	defer server.Close()

	d := NewDriftiveNotification(server.URL, "secret-token")
	resp, err := d.Handle(context.Background(), sampleResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/api/v1/drift_analysis" {
		t.Errorf("expected path /api/v1/drift_analysis, got %s", gotPath)
	}
	if gotToken != "secret-token" {
		t.Errorf("expected X-Token 'secret-token', got %q", gotToken)
	}
	if gotKey == "" {
		t.Error("expected a non-empty Idempotency-Key header")
	}
	if resp == nil || resp.DashboardURL != "http://dash/run/abc" {
		t.Errorf("expected the dashboard URL to be parsed, got %+v", resp)
	}

	// The project dir must reach the API exactly as the analysis reported it — the API
	// stores this string verbatim and the dashboard renders it as the project name.
	var sent drift.DriftDetectionResult
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("failed to unmarshal sent body: %v", err)
	}
	if len(sent.ProjectResults) != 1 {
		t.Fatalf("expected 1 project result, got %d", len(sent.ProjectResults))
	}
	if sent.ProjectResults[0].Project.Dir != "infra/foo.bar" {
		t.Errorf("expected dir 'infra/foo.bar', got %q", sent.ProjectResults[0].Project.Dir)
	}
}

func TestHandle_ReturnsErrorOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad payload"))
	}))
	defer server.Close()

	d := NewDriftiveNotification(server.URL, "secret-token")
	resp, err := d.Handle(context.Background(), sampleResult())
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if resp != nil {
		t.Errorf("expected a nil response alongside the error, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "bad payload") {
		t.Errorf("expected the server body in the error, got %v", err)
	}
}

func TestHandle_ReturnsErrorOnUnreachableServer(t *testing.T) {
	// Bounded by a deadline rather than left to exhaust the retry backoff, which would make
	// this a ~15s test.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	d := NewDriftiveNotification("http://127.0.0.1:59999", "secret-token")
	if _, err := d.Handle(ctx, sampleResult()); err == nil {
		t.Error("expected an error when the API is unreachable")
	}
}

// TestHandle_ReusesIdempotencyKeyAcrossRetries pins the property that makes retrying safe:
// every attempt carries the same key, so a retry of a request the server already accepted
// returns the existing run instead of creating a duplicate.
func TestHandle_ReusesIdempotencyKeyAcrossRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("retry backoff makes this slow; skipped under -short")
	}

	var mu sync.Mutex
	keys := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		attempt := len(keys)
		mu.Unlock()

		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"run_id":"abc","dashboard_url":"http://dash/run/abc"}`))
	}))
	defer server.Close()

	d := NewDriftiveNotification(server.URL, "secret-token")
	if _, err := d.Handle(context.Background(), sampleResult()); err != nil {
		t.Fatalf("expected the retry to succeed, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(keys) < 2 {
		t.Fatalf("expected the 500 to be retried, got %d request(s)", len(keys))
	}
	if keys[0] == "" {
		t.Error("expected a non-empty Idempotency-Key")
	}
	for i, k := range keys {
		if k != keys[0] {
			t.Errorf("attempt %d used key %q, expected %q from the first attempt", i+1, k, keys[0])
		}
	}
}
