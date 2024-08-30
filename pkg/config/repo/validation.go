package repo

import (
	"errors"
	"github.com/rs/zerolog/log"
)

var ErrMissingRepoConfig = "missing repository config"
var ErrInvalidLabelName = "invalid label name"
var ErrConflictingLabels = "conflicting drift and error labels"

func ValidateRepoConfig(repoConfig *DriftiveRepoConfig) {
	//nolint:staticcheck
	if nil == repoConfig {
		log.Fatal().Err(errors.New(ErrMissingRepoConfig)).Msg("Repository config is required. Please create a .driftive.y(a)ml file in the root of the repository.")
	}
	//nolint:staticcheck
	if nil != repoConfig.GitHub.Issues.Labels {
		for _, label := range repoConfig.GitHub.Issues.Labels {
			if label == "" {
				log.Fatal().Err(errors.New(ErrInvalidLabelName)).Msgf("Invalid label name: %s", label)
			}
			if repoConfig.GitHub.Issues.Errors.Enabled && repoConfig.GitHub.Issues.Errors.Labels != nil {
				for _, errorLabel := range repoConfig.GitHub.Issues.Errors.Labels {
					if errorLabel == "" {
						log.Fatal().Err(errors.New(ErrInvalidLabelName)).Msgf("Invalid label name: %s", errorLabel)
					}
					if errorLabel == label {
						log.Fatal().Err(errors.New(ErrConflictingLabels)).Msgf("Label '%s' is used for both drift and error issues", errorLabel)
					}
				}
			}
		}
	}
}
