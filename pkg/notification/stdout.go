package notification

import (
	"driftive/pkg/drift"
	"github.com/rs/zerolog/log"
)

type Stdout struct {
}

func NewStdout() Stdout {
	return Stdout{}
}

func (stdout Stdout) Send(driftResult drift.DriftDetectionResult) error {
	if driftResult.TotalDrifted == 0 {
		return nil
	}
	log.Info().Msgf("============================================")
	log.Info().Msgf("Analysis completed in %s", driftResult.Duration)
	log.Info().Msgf("State Drift detected in Terragrunt projects")
	log.Info().Msgf("Drifts %d out of %d total projects", driftResult.TotalDrifted, driftResult.TotalProjects)
	log.Info().Msgf("Projects with drift:")
	for _, project := range driftResult.DriftedProjects {
		log.Info().Msgf("Project: %s", project.Project)
	}
	log.Info().Msgf("============================================")
	return nil
}
