package github

import (
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/models"
	"errors"
	"github.com/google/go-github/v64/github"
	"github.com/rs/zerolog/log"
)

type GithubIssueNotification struct {
	config     *config.DriftiveConfig
	repoConfig *repo.DriftiveRepoConfig
	ghClient   *github.Client
}

type GithubIssue struct {
	Title   string
	Body    string
	Labels  []string
	Project models.Project
}

func NewGithubIssueNotification(config *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) (*GithubIssueNotification, error) {
	if config.GithubContext.Repository == "" || config.GithubContext.RepositoryOwner == "" {
		log.Warn().Msg("Github repository or owner not provided. Skipping github notification")
		return nil, errors.New(ErrRepoNotProvided)
	}

	if config.GithubToken == "" {
		log.Warn().Msg("Github token not provided. Skipping github notification")
		return nil, errors.New(ErrGHTokenNotProvided)
	}

	ghClient := github.NewClient(nil).WithAuthToken(config.GithubToken)
	return &GithubIssueNotification{config: config, repoConfig: repoConfig, ghClient: ghClient}, nil
}
