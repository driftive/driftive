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

func (s *SCMNoop) GetChangedFilesForAllPRs(ctx context.Context) ([]string, error) {
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
