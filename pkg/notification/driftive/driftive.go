package driftive

import (
	"context"
	"driftive/pkg/drift"
	"encoding/json"
	"fmt"

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

// Handle sends drift analysis results to the Driftive API and returns the dashboard URL if available
func (d Driftive) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) (*AnalysisResponse, error) {
	client := resty.New()
	defer client.Close()

	res, err := client.R().
		WithContext(ctx).
		SetHeader("X-Token", d.Token).
		SetBody(driftResult).
		Post(d.Url + "/api/v1/drift_analysis")

	if err != nil {
		return nil, err
	}

	if res.StatusCode() != 200 {
		log.Error().Msgf("Failed to send drift analysis result to driftive api. Invalid token?. Response: %v", res.String())
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
