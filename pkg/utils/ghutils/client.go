package ghutils

import (
	"errors"
	"github.com/google/go-github/v88/github"
	"github.com/rs/zerolog/log"
)

const (
	ErrGHTokenNotProvided = "github token not provided"
)

func GitHubClient(githubToken string) (*github.Client, error) {
	if githubToken == "" {
		log.Warn().Msg("Github token not provided. Skipping github notification")
		return nil, errors.New(ErrGHTokenNotProvided)
	}

	return github.NewClient(github.WithAuthToken(githubToken))
}
