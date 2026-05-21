package driftive

import (
	"context"
	"driftive/pkg/drift"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"resty.dev/v3"
)

// AnalysisResponse is the response from the Driftive API after uploading analysis results
type AnalysisResponse struct {
	RunID        string `json:"run_id"`
	DashboardURL string `json:"dashboard_url"`
}

type Driftive struct {
	Url   string
	Token string
}

func NewDriftiveNotification(url, token string) Driftive {
	return Driftive{
		Url:   url,
		Token: token,
	}
}

// Handle sends drift analysis results to the Driftive API and returns the dashboard URL if available.
// Transient failures (network errors, 5xx, 429) are retried with exponential backoff. The same
// Idempotency-Key is sent on every attempt so a retry of a request the server already accepted
// returns the existing run instead of creating a duplicate.
func (d Driftive) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) (*AnalysisResponse, error) {
	idemKey := uuid.NewString()

	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(15 * time.Second).
		AddRetryConditions(func(res *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			sc := res.StatusCode()
			return sc == 429 || sc >= 500
		})
	defer client.Close()

	res, err := client.R().
		WithContext(ctx).
		SetHeader("X-Token", d.Token).
		SetHeader("Idempotency-Key", idemKey).
		SetBody(driftResult).
		Post(d.Url + "/api/v1/drift_analysis")

	if err != nil {
		return nil, err
	}

	if res.StatusCode() != 200 {
		if res.StatusCode() == 401 || res.StatusCode() == 403 {
			log.Error().Msgf("Driftive API rejected token (status %d). Response: %s", res.StatusCode(), res.String())
		} else {
			log.Error().Msgf("Driftive API returned non-200 (status %d) after retries. Response: %s", res.StatusCode(), res.String())
		}
		return nil, fmt.Errorf("failed to send drift analysis result: %s", res.String())
	}

	// Parse response to get dashboard URL
	var response AnalysisResponse
	if err := json.Unmarshal(res.Bytes(), &response); err != nil {
		log.Warn().Msgf("Failed to parse driftive api response: %v", err)
		// Return empty response - upload succeeded but response parsing failed
		return &AnalysisResponse{}, nil
	}

	return &response, nil
}
