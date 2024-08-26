package backend

// DriftIssuesState represents the state of current issues detected.
type DriftIssuesState struct {
	NumOpenIssues     int
	NumResolvedIssues int
	StateUpdated      bool
}
