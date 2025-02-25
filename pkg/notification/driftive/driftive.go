package driftive

import (
	"context"
	"driftive/pkg/drift"
	"github.com/rs/zerolog/log"
	"resty.dev/v3"
)

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

func (d Driftive) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) error {

	client := resty.New()
	defer client.Close()

	res, err := client.R().
		WithContext(ctx).
		SetHeader("X-Token", d.Token).
		SetBody(driftResult).
		Post(d.Url + "/api/v1/drift_analysis")

	if err != nil {
		return err
	}

	if res.StatusCode() != 200 {
		log.Error().Msgf("Failed to send drift analysis result to driftive api. Invalid token?. Response: %v", res.String())
	}

	return nil
}
