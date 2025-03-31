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
