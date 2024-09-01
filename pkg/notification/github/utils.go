package github

import "driftive/pkg/notification/github/types"

func filterIssues(issues []types.ProjectIssue, issuesToRemove []types.ProjectIssue) []types.ProjectIssue {
	var filteredIssues []types.ProjectIssue
	for _, issue := range issues {
		if !containsIssue(issuesToRemove, issue) {
			filteredIssues = append(filteredIssues, issue)
		}
	}
	return filteredIssues
}

func containsIssue(issues []types.ProjectIssue, issue types.ProjectIssue) bool {
	for _, i := range issues {
		if i.Project.Dir == issue.Project.Dir && i.Kind == issue.Kind {
			return true
		}
	}
	return false
}
