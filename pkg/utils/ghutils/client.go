package ghutils

import (
	"errors"
	"github.com/google/go-github/v81/github"
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

	ghClient := github.NewClient(nil).WithAuthToken(githubToken)
	return ghClient, nil
}
