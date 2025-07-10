package noop

import (
	"context"
	"driftive/pkg/notification/github/types"
	"driftive/pkg/vcs/vcstypes"
)

type SCMNoop struct {
}

func NewSCMNoop() *SCMNoop {
	return &SCMNoop{}
}

func (s *SCMNoop) GetAllOpenRepoIssues(ctx context.Context) ([]*vcstypes.VCSIssue, error) {
	return make([]*vcstypes.VCSIssue, 0), nil
}

func (s *SCMNoop) GetChangedFilesForAllOpenPrs(ctx context.Context, allOpenPrs []*vcstypes.VCSPullRequest) ([]string, error) {
	return make([]string, 0), nil
}

func (s *SCMNoop) CreateOrUpdateIssue(ctx context.Context, driftiveIssue types.GithubIssue, openIssues []*vcstypes.VCSIssue, updateOnly bool) vcstypes.CreateOrUpdateResult {
	return vcstypes.CreateOrUpdateResult{
		Created:     false,
		RateLimited: false,
		Issue:       nil,
	}
}

func (s *SCMNoop) CreateIssueComment(ctx context.Context, issueNumber int) error {
	return nil
}

func (s *SCMNoop) CloseIssue(ctx context.Context, issueNumber int) error {
	return nil
}

func (s *SCMNoop) GetAllOpenPRs(ctx context.Context) ([]*vcstypes.VCSPullRequest, error) {
	return make([]*vcstypes.VCSPullRequest, 0), nil
}

func (s *SCMNoop) BranchExists(ctx context.Context, branchName string) (bool, error) {
	return false, nil
}

func (s *SCMNoop) CreateBranch(ctx context.Context, branchName string) error {
	return nil
}

func (s *SCMNoop) AddFileToBranch(ctx context.Context, branchName string, filePath string, content string, commitMessage string) error {
	return nil
}

func (s *SCMNoop) CreateOrUpdatePullRequest(ctx context.Context, driftivePullRequest types.GithubPullRequest, updateOnly bool) vcstypes.CreateOrUpdatePullRequestResult {
	return vcstypes.CreateOrUpdatePullRequestResult{
		Created:     false,
		RateLimited: false,
		PullRequest: nil,
	}
}

func (s *SCMNoop) CreatePullRequestComment(ctx context.Context, pullRequestNumber int, comment string) error {
	return nil
}

func (s *SCMNoop) ClosePullRequest(ctx context.Context, pullRequestNumber int) error {
	return nil
}
