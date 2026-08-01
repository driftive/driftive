package console

import (
	"context"
	"driftive/pkg/drift"
	"driftive/pkg/notification/report"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Stdout struct {
	logger zerolog.Logger
}

func NewStdout() Stdout {
	return Stdout{logger: log.Logger}
}

func (s Stdout) Handle(ctx context.Context, driftResult drift.DriftDetectionResult) error {
	summary := report.Classify(driftResult)

	s.logger.Info().Msgf("============================================")
	s.logger.Info().Msgf("Analysis completed in %s", summary.DurationText())
	s.logger.Info().Msgf("%d projects: %d drifted, %d errored, %d skipped, %d clean",
		summary.TotalProjects, summary.NumDrifted(), summary.NumErrored(), summary.NumSkipped(), summary.NumClean())

	if summary.NotChecked > 0 {
		s.logger.Info().Msgf("%d projects were not checked", summary.NotChecked)
	}

	s.section("Projects with state drift:", summary.Drifted)
	s.section("Projects that failed to analyze:", summary.Errored)
	s.section("Skipped due to open PRs:", summary.Skipped)

	if !summary.HasFindings() {
		s.logger.Info().Msg("No drift or errors detected.")
	}

	s.logger.Info().Msgf("============================================")
	return nil
}

func (s Stdout) section(title string, projects []report.Project) {
	if len(projects) == 0 {
		return
	}
	s.logger.Info().Msg(title)
	for _, p := range projects {
		s.logger.Info().Msg("  - " + describe(p))
	}
}

func describe(p report.Project) string {
	if p.FailedPhase == "" {
		return p.Dir
	}
	return fmt.Sprintf("%s (%s)", p.Dir, p.FailedPhase)
}
