package github

func IssueKindToString(kind IssueKind) string {
	switch kind {
	case DriftIssueKind:
		return "drift"
	case ErrorIssueKind:
		return "error"
	default:
		return "unknown"
	}
}

func filterIssues(issues []ProjectIssue, issuesToRemove []ProjectIssue) []ProjectIssue {
	var filteredIssues []ProjectIssue
	for _, issue := range issues {
		if !containsIssue(issuesToRemove, issue) {
			filteredIssues = append(filteredIssues, issue)
		}
	}
	return filteredIssues

}

func containsIssue(issues []ProjectIssue, issue ProjectIssue) bool {
	for _, i := range issues {
		if i.Project.Dir == issue.Project.Dir && i.Kind == issue.Kind {
			return true
		}
	}
	return false
}
