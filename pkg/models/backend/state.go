package backend

// DriftIssuesState represents the state of drift issues.
type DriftIssuesState struct {
	NumOpenIssues     int
	NumResolvedIssues int
	StateUpdated      bool
}
