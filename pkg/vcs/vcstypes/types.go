package vcstypes

type VCSIssue struct {
	Body   string `json:"body"`
	Title  string `json:"title"`
	Number int    `json:"number"`
}

type CreateOrUpdateResult struct {
	Created     bool
	RateLimited bool
	Issue       *VCSIssue
}
