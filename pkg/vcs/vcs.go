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
	// GetChangedFilesForAllOpenPrs returns all changed files for all open PRs
	GetChangedFilesForAllOpenPrs(ctx context.Context, allOpenPrs []*vcstypes.VCSPullRequest) ([]string, error)
	CreateOrUpdateIssue(ctx context.Context, driftiveIssue types.GithubIssue,
		openIssues []*vcstypes.VCSIssue, updateOnly bool) vcstypes.CreateOrUpdateResult
	CreateIssueComment(ctx context.Context, issueNumber int) error
	CloseIssue(ctx context.Context, issueNumber int) error
	// PR specific methods
	GetAllOpenPRs(ctx context.Context) ([]*vcstypes.VCSPullRequest, error)
	BranchExists(ctx context.Context, branchName string) (bool, error)
	CreateBranch(ctx context.Context, branchName string) error
	AddFileToBranch(ctx context.Context, branchName string, filePath string, content string, commitMessage string) error
	CreateOrUpdatePullRequest(ctx context.Context, driftivePullRequest types.GithubPullRequest, updateOnly bool) vcstypes.CreateOrUpdatePullRequestResult
	CreatePullRequestComment(ctx context.Context, pullRequestNumber int, comment string) error
	ClosePullRequest(ctx context.Context, pullRequestNumber int) error
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
