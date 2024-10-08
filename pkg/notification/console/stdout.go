package console

import (
	"context"
	"driftive/pkg/drift"
	"github.com/rs/zerolog/log"
)

type Stdout struct {
}

func NewStdout() Stdout {
	return Stdout{}
}

func (s Stdout) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) error {
	if driftResult.TotalDrifted == 0 {
		return nil
	}
	log.Info().Msgf("============================================")
	log.Info().Msgf("Analysis completed in %s", driftResult.Duration)
	log.Info().Msgf("State Drift detected in projects")
	log.Info().Msgf("Drifts %d out of %d total projects", driftResult.TotalDrifted, driftResult.TotalProjects)
	log.Info().Msgf("Projects with state drift:")
	for _, project := range driftResult.ProjectResults {
		if project.Drifted {
			log.Info().Msgf("Project: %s", project.Project.Dir)
		}
	}
	log.Info().Msgf("============================================")
	return nil
}
