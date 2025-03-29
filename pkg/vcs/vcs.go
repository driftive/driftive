package vcs

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/config/repo"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/utils/ghutils"
	"driftive/pkg/vcs/github"
	"driftive/pkg/vcs/noop"
	"driftive/pkg/vcs/vcstypes"
)

type VCS interface {
	// GetAllOpenRepoIssues returns all open issues for the repository
	GetAllOpenRepoIssues(ctx context.Context) ([]*vcstypes.VCSIssue, error)
	// GetChangedFilesForAllPRs returns all changed files for all open PRs
	GetChangedFilesForAllPRs(ctx context.Context) ([]string, error)
	CreateOrUpdateIssue(ctx context.Context, driftiveIssue types.GithubIssue,
		openIssues []*vcstypes.VCSIssue, updateOnly bool) vcstypes.CreateOrUpdateResult
}

func NewVCS(cfg *config.DriftiveConfig, repoConfig *repo.DriftiveRepoConfig) (VCS, error) {
	if cfg.GithubContext.IsValid() && cfg.GithubToken != "" {
		ghClient, err := ghutils.GitHubClient(cfg.GithubToken)
		if err != nil {
			return nil, err
		}
		return github.NewGHOps(cfg, repoConfig, ghClient), nil
	}

	return noop.NewSCMNoop(), nil
}
