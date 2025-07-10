package vcstypes

type VCSIssue struct {
	Body   string `json:"body,omitempty"`
	Title  string `json:"title,omitempty"`
	Number int    `json:"number"`
}

type CreateOrUpdateResult struct {
	Created     bool
	RateLimited bool
	Issue       *VCSIssue
}

type VCSPullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Url    string `json:"url"`
	Body   string `json:"body,omitempty"`
}

type CreateOrUpdatePullRequestResult struct {
	Created     bool
	RateLimited bool
	PullRequest *VCSPullRequest
}
