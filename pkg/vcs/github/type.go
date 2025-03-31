package github

import (
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"github.com/google/go-github/v69/github"
)

type GHOps struct {
	config     *config.DriftiveConfig
	repoConfig *repo.DriftiveRepoConfig
	ghClient   *github.Client
}

func NewGHOps(cfg *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig, ghClient *github.Client) *GHOps {
	return &GHOps{
		config:     cfg,
		repoConfig: repoConfig,
		ghClient:   ghClient,
	}
}
