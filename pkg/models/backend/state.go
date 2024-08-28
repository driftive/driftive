package backend

// DriftIssuesState represents the state of current issues detected.
// TODO add support for storing this state encoded in a Github issue
type DriftIssuesState struct {
	NumOpenIssues          int
	NumResolvedIssues      int
	NumOpenErrorIssues     int
	NumResolvedErrorIssues int
	StateUpdated           bool
}
